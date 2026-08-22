package sql

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"taproot/storage"
)

type ColumnType int

const (
	TypeInt ColumnType = iota
	TypeText
	TypeFloat
	TypeBool
)

func (t ColumnType) String() string {
	switch t {
	case TypeInt:
		return "INT"
	case TypeText:
		return "TEXT"
	case TypeFloat:
		return "FLOAT"
	case TypeBool:
		return "BOOL"
	default:
		return "UNKNOWN"
	}
}

// ParseColumnType accepts a handful of common aliases for each type.
func ParseColumnType(s string) (ColumnType, error) {
	switch strings.ToUpper(s) {
	case "INT", "INTEGER":
		return TypeInt, nil
	case "TEXT", "STRING", "VARCHAR":
		return TypeText, nil
	case "FLOAT", "REAL", "DOUBLE":
		return TypeFloat, nil
	case "BOOL", "BOOLEAN":
		return TypeBool, nil
	default:
		return 0, fmt.Errorf("unknown column type %q", s)
	}
}

type Column struct {
	Name       string
	Type       ColumnType
	PrimaryKey bool
}

// TableSchema describes one table. Every table has exactly one INT
// primary key column, since that's what a storage.Tree can key rows by.
// If CREATE TABLE doesn't declare one, an implicit "rowid" column is
// added and auto-incremented on insert.
type TableSchema struct {
	Name       string
	Columns    []Column
	PrimaryKey string
	NextRowID  int
}

func (s *TableSchema) Column(name string) (Column, bool) {
	for _, c := range s.Columns {
		if strings.EqualFold(c.Name, name) {
			return c, true
		}
	}
	return Column{}, false
}

// Catalog tracks every table's schema and its on-disk B+Tree. It is
// itself persisted to disk (as catalog.gob in dir) so tables survive a
// restart; each table's rows live in their own "<dir>/<table>.db" file
// managed by the storage package.
type Catalog struct {
	dir    string
	Tables map[string]*TableSchema
	trees  map[string]*storage.Tree
}

const catalogFileName = "catalog.gob"

// OpenCatalog loads an existing catalog rooted at dir, or creates a fresh
// one if none exists yet. Table trees are loaded lazily on first use.
func OpenCatalog(dir string) (*Catalog, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create catalog directory: %w", err)
	}

	cat := &Catalog{
		dir:    dir,
		Tables: make(map[string]*TableSchema),
		trees:  make(map[string]*storage.Tree),
	}

	data, err := os.ReadFile(filepath.Join(dir, catalogFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return cat, nil // brand new database
		}
		return nil, fmt.Errorf("read catalog: %w", err)
	}

	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&cat.Tables); err != nil {
		return nil, fmt.Errorf("decode catalog: %w", err)
	}

	return cat, nil
}

func (c *Catalog) persist() error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(c.Tables); err != nil {
		return fmt.Errorf("encode catalog: %w", err)
	}
	if err := os.WriteFile(filepath.Join(c.dir, catalogFileName), buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write catalog: %w", err)
	}
	return nil
}

func (c *Catalog) treePath(table string) string {
	return filepath.Join(c.dir, table+".db")
}

// CreateTable registers a new table and creates its backing tree on disk.
// Fails if a table with that name (case-insensitively) already exists.
func (c *Catalog) CreateTable(name string, columns []Column) (*TableSchema, error) {
	key := strings.ToLower(name)
	if _, exists := c.Tables[key]; exists {
		return nil, fmt.Errorf("table %q already exists", name)
	}

	hasPK := false
	for _, col := range columns {
		if col.PrimaryKey {
			if col.Type != TypeInt {
				return nil, fmt.Errorf("primary key column %q must be INT", col.Name)
			}
			hasPK = true
		}
	}

	schema := &TableSchema{Name: name, Columns: columns}
	if hasPK {
		for _, col := range columns {
			if col.PrimaryKey {
				schema.PrimaryKey = col.Name
				break
			}
		}
	} else {
		schema.Columns = append([]Column{{Name: "rowid", Type: TypeInt, PrimaryKey: true}}, columns...)
		schema.PrimaryKey = "rowid"
	}

	c.Tables[key] = schema
	c.trees[key] = storage.NewTree()

	if err := storage.SaveTree(c.trees[key], c.treePath(name)); err != nil {
		return nil, fmt.Errorf("create table storage: %w", err)
	}
	if err := c.persist(); err != nil {
		return nil, err
	}

	return schema, nil
}

// DropTable removes a table's schema and deletes its on-disk tree.
func (c *Catalog) DropTable(name string) error {
	key := strings.ToLower(name)
	if _, exists := c.Tables[key]; !exists {
		return fmt.Errorf("table %q does not exist", name)
	}

	delete(c.Tables, key)
	delete(c.trees, key)

	if err := os.Remove(c.treePath(name)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove table storage: %w", err)
	}
	return c.persist()
}

// GetSchema returns the schema for name, case-insensitively.
func (c *Catalog) GetSchema(name string) (*TableSchema, error) {
	schema, ok := c.Tables[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("table %q does not exist", name)
	}
	return schema, nil
}

// GetTree returns the (lazily loaded) tree backing table name.
func (c *Catalog) GetTree(name string) (*storage.Tree, error) {
	key := strings.ToLower(name)
	schema, ok := c.Tables[key]
	if !ok {
		return nil, fmt.Errorf("table %q does not exist", name)
	}
	if tree, ok := c.trees[key]; ok {
		return tree, nil
	}

	tree, err := storage.LoadTree(c.treePath(schema.Name))
	if err != nil {
		return nil, fmt.Errorf("load table storage: %w", err)
	}
	c.trees[key] = tree
	return tree, nil
}

// TableNames lists every table alphabetically, for SHOW TABLES.
func (c *Catalog) TableNames() []string {
	names := make([]string, 0, len(c.Tables))
	for _, schema := range c.Tables {
		names = append(names, schema.Name)
	}
	sort.Strings(names)
	return names
}

// Flush persists the catalog metadata and every loaded table's tree.
// storage.SaveTree only writes pages that actually changed, so calling
// this after every statement is cheap even with many tables loaded.
func (c *Catalog) Flush() error {
	for key, tree := range c.trees {
		schema := c.Tables[key]
		if err := storage.SaveTree(tree, c.treePath(schema.Name)); err != nil {
			return fmt.Errorf("save table %q: %w", schema.Name, err)
		}
	}
	return c.persist()
}