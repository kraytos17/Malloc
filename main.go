package main

import malloc "github.com/kraytos17/Malloc/malloc"

func main() {
	malloc.InitHeap()
	ptr := malloc.HeapAlloc(123)
	malloc.AllocedChunks.Dump("Allocated")
	malloc.HeapFree(ptr)
	malloc.FreedChunks.Dump("Freed")
	malloc.HeapCollect()
	malloc.FreedChunks.Dump("Freed after GC")
}
