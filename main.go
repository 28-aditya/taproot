package main

import ("taproot/server"
		"taproot/storage"
)

func main() {
	storage.Check()
	server.Start()
}
