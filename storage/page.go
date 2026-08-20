package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"os"
)

const (
	pageSize     = 4096
	magicNumber  = 0x54415052 // "TAPR"
	headerPageID = 0
	nilPageID    = 0 // 0 is reserved for the header page, so it doubles as "no page"
)

func init() {

	gob.Register(int(0))
	gob.Register(int64(0))
	gob.Register(float64(0))
	gob.Register(string(""))
	gob.Register(bool(false))
	gob.Register([]byte(nil))
}


type fileHeader struct {
	Magic        uint32
	RootPageID   uint32
	NumPages     uint32 // total pages allocated in the file, including the header page
	FreeListHead uint32 // head of a singly linked free-page list, 0 if empty
}

type nodePayload struct {
	IsLeaf   bool
	Keys     []int
	Values   []any    // leaf only
	NextID   uint32   // leaf only, sibling page ID (0 = none)
	ChildIDs []uint32 // internal only
}

type Pager struct {
	file   *os.File
	header fileHeader
}

func OpenPager(path string) (*Pager, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("open database file: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("stat database file: %w", err)
	}

	pager := &Pager{file: file}

	if info.Size() == 0 {
		pager.header = fileHeader{Magic: magicNumber, NumPages: 1}
		if err := pager.flushHeader(); err != nil {
			file.Close()
			return nil, err
		}
		return pager, nil
	}

	if err := pager.readGobPage(headerPageID, &pager.header); err != nil {
		file.Close()
		return nil, fmt.Errorf("read database header: %w", err)
	}
	if pager.header.Magic != magicNumber {
		file.Close()
		return nil, fmt.Errorf("%s is not a taproot database file", path)
	}

	return pager, nil
}

func (p *Pager) Close() error {
	if err := p.flushHeader(); err != nil {
		p.file.Close()
		return err
	}
	return p.file.Close()
}

func (p *Pager) flushHeader() error {
	return p.writeGobPage(headerPageID, &p.header)
}

func (p *Pager) AllocatePage() (uint32, error) {
	if p.header.FreeListHead != nilPageID {
		id := p.header.FreeListHead

		raw, err := p.readRawPage(id)
		if err != nil {
			return 0, err
		}
		p.header.FreeListHead = binary.LittleEndian.Uint32(raw[0:4])

		if err := p.flushHeader(); err != nil {
			return 0, err
		}
		return id, nil
	}

	id := p.header.NumPages
	p.header.NumPages++
	if err := p.flushHeader(); err != nil {
		return 0, err
	}
	return id, nil
}

func (p *Pager) FreePage(id uint32) error {
	if id == nilPageID {
		return nil
	}

	buf := make([]byte, pageSize)
	binary.LittleEndian.PutUint32(buf[0:4], p.header.FreeListHead)
	if err := p.writeRawPage(id, buf); err != nil {
		return err
	}

	p.header.FreeListHead = id
	return p.flushHeader()
}

func (p *Pager) readRawPage(id uint32) ([]byte, error) {
	buf := make([]byte, pageSize)
	offset := int64(id) * int64(pageSize)

	_, err := p.file.ReadAt(buf, offset)
	if err != nil && err.Error() != "EOF" {
		return nil, fmt.Errorf("read page %d: %w", id, err)
	}

	return buf, nil
}

func (p *Pager) writeRawPage(id uint32, data []byte) error {
	if len(data) != pageSize {
		return fmt.Errorf("write page %d: page must be exactly %d bytes, got %d", id, pageSize, len(data))
	}
	offset := int64(id) * int64(pageSize)
	if _, err := p.file.WriteAt(data, offset); err != nil {
		return fmt.Errorf("write page %d: %w", id, err)
	}
	return nil
}

func (p *Pager) writeGobPage(id uint32, v any) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		return fmt.Errorf("encode page %d: %w", id, err)
	}

	data := buf.Bytes()
	if len(data)+4 > pageSize {
		return fmt.Errorf("encode page %d: encoded size %d exceeds page size %d", id, len(data)+4, pageSize)
	}

	page := make([]byte, pageSize)
	binary.LittleEndian.PutUint32(page[0:4], uint32(len(data)))
	copy(page[4:], data)

	return p.writeRawPage(id, page)
}

func (p *Pager) readGobPage(id uint32, v any) error {
	raw, err := p.readRawPage(id)
	if err != nil {
		return err
	}

	length := binary.LittleEndian.Uint32(raw[0:4])
	if length == 0 {
		return fmt.Errorf("read page %d: page is empty", id)
	}
	if int(length) > pageSize-4 {
		return fmt.Errorf("read page %d: corrupt length prefix %d", id, length)
	}

	if err := gob.NewDecoder(bytes.NewReader(raw[4 : 4+length])).Decode(v); err != nil {
		return fmt.Errorf("decode page %d: %w", id, err)
	}
	return nil
}

func (p *Pager) WriteNode(id uint32, node *Node, childIDs []uint32, nextID uint32) error {
	payload := nodePayload{
		IsLeaf: node.IsLeaf,
		Keys:   node.Keys,
	}
	if node.IsLeaf {
		payload.Values = node.Values
		payload.NextID = nextID
	} else {
		payload.ChildIDs = childIDs
	}
	return p.writeGobPage(id, &payload)
}

func (p *Pager) ReadNode(id uint32) (node *Node, childIDs []uint32, nextID uint32, err error) {
	var payload nodePayload
	if err := p.readGobPage(id, &payload); err != nil {
		return nil, nil, 0, err
	}

	node = &Node{
		IsLeaf: payload.IsLeaf,
		Keys:   payload.Keys,
		Values: payload.Values,
	}
	return node, payload.ChildIDs, payload.NextID, nil
}

func SaveTree(tree *bPlusTree, path string) error {
	pager, err := OpenPager(path)
	if err != nil {
		return err
	}

	rootID, err := pager.saveNodeRec(tree.rootNode)
	if err != nil {
		pager.file.Close()
		return err
	}
	pager.header.RootPageID = rootID

	return pager.Close()
}

func (p *Pager) saveNodeRec(node *Node) (uint32, error) {
	if node == nil {
		return nilPageID, nil
	}

	if !node.dirty && node.pageID != nilPageID {
		return node.pageID, nil
	}

	id := node.pageID
	if id == nilPageID {
		var err error
		id, err = p.AllocatePage()
		if err != nil {
			return 0, err
		}
		node.pageID = id
	}

	var childIDs []uint32
	var nextID uint32
	var err error

	if node.IsLeaf {
		nextID, err = p.saveNodeRec(node.Next)
		if err != nil {
			return 0, err
		}
	} else {
		childIDs = make([]uint32, len(node.Pointers))
		for i, child := range node.Pointers {
			childIDs[i], err = p.saveNodeRec(child)
			if err != nil {
				return 0, err
			}
		}
	}

	if err := p.WriteNode(id, node, childIDs, nextID); err != nil {
		return 0, err
	}
	node.dirty = false

	return id, nil
}

func LoadTree(path string) (*bPlusTree, error) {
	pager, err := OpenPager(path)
	if err != nil {
		return nil, err
	}
	defer pager.Close()

	if pager.header.RootPageID == nilPageID {
		return NewTree(), nil
	}

	loaded := make(map[uint32]*Node)
	root, err := pager.loadNodeRec(pager.header.RootPageID, loaded)
	if err != nil {
		return nil, err
	}

	return &bPlusTree{rootNode: root}, nil
}

func (p *Pager) loadNodeRec(id uint32, loaded map[uint32]*Node) (*Node, error) {
	if id == nilPageID {
		return nil, nil
	}
	if node, ok := loaded[id]; ok {
		return node, nil
	}

	node, childIDs, nextID, err := p.ReadNode(id)
	if err != nil {
		return nil, err
	}
	node.pageID = id
	loaded[id] = node // reserve before recursing in case of shared references

	if node.IsLeaf {
		next, err := p.loadNodeRec(nextID, loaded)
		if err != nil {
			return nil, err
		}
		node.Next = next
	} else {
		pointers := make([]*Node, len(childIDs))
		for i, cid := range childIDs {
			child, err := p.loadNodeRec(cid, loaded)
			if err != nil {
				return nil, err
			}
			pointers[i] = child
		}
		node.Pointers = pointers
	}

	return node, nil
}