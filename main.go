package main

import (
	"fmt"
	"unsafe"
)

const (
	HEAP_CAP_BYTES = 640_000
	HEAP_CAP_WORDS = HEAP_CAP_BYTES / int(unsafe.Sizeof(uintptr(0)))
	CHUNK_LIST_CAP = 1024
)

var (
	heap          [HEAP_CAP_WORDS]uintptr
	stackBase     uintptr
	reachable     [CHUNK_LIST_CAP]bool
	toFree        [CHUNK_LIST_CAP]unsafe.Pointer
	toFreeCount   int
	allocedChunks = ChunkList{chunks: make([]Chunk, 0, CHUNK_LIST_CAP)}
	freedChunks   = ChunkList{
		count:  1,
		chunks: []Chunk{{start: uintptr(unsafe.Pointer(&heap[0])), size: uintptr(HEAP_CAP_WORDS)}},
	}
	tmpChunks = ChunkList{chunks: make([]Chunk, 0, CHUNK_LIST_CAP)}
)

type Chunk struct {
	start uintptr
	size  uintptr
}

type ChunkList struct {
	count  int
	chunks []Chunk
}

func (list *ChunkList) insert(start uintptr, size uintptr) {
	if list.count >= CHUNK_LIST_CAP {
		panic("Chunk list capacity exceeded")
	}

	newChunk := Chunk{start: start, size: size}
	if list.count < len(list.chunks) {
		list.chunks[list.count] = newChunk
	} else {
		list.chunks = append(list.chunks, newChunk)
	}
	list.count++
}

func (list *ChunkList) merge(src *ChunkList) {
	list.count = 0

	for i := 0; i < src.count; i++ {
		chunk := src.chunks[i]
		if list.count > 0 {
			topChunk := &list.chunks[list.count-1]
			if topChunk.start+topChunk.size == chunk.start {
				topChunk.size += chunk.size
			} else {
				list.insert(chunk.start, chunk.size)
			}
		} else {
			list.insert(chunk.start, chunk.size)
		}
	}
}

func (list *ChunkList) dump(name string) {
	fmt.Printf("%s Chunks (%d):\n", name, list.count)
	for i := 0; i < list.count; i++ {
		chunk := list.chunks[i]
		fmt.Printf("  start: %p, size: %d\n", unsafe.Pointer(chunk.start), chunk.size)
	}
}

func (list *ChunkList) find(ptr uintptr) int {
	for i := 0; i < list.count; i++ {
		if list.chunks[i].start == ptr {
			return i
		}
	}
	return -1
}

func (list *ChunkList) remove(index int) {
	if index < 0 || index >= list.count {
		panic("Index out of bounds")
	}

	copy(list.chunks[index:], list.chunks[index+1:list.count])
	list.count--
}

func heapAlloc(sizeBytes uintptr) uintptr {
	sizeWords := (sizeBytes + unsafe.Sizeof(uintptr(0)) - 1) / unsafe.Sizeof(uintptr(0))

	if sizeWords > 0 {
		tmpChunks.merge(&freedChunks)
		freedChunks = tmpChunks

		for i := 0; i < freedChunks.count; i++ {
			chunk := freedChunks.chunks[i]
			if chunk.size >= sizeWords {
				freedChunks.remove(i)

				tailSizeWords := chunk.size - sizeWords
				allocedChunks.insert(chunk.start, sizeWords)

				if tailSizeWords > 0 {
					freedChunks.insert(chunk.start+sizeWords*unsafe.Sizeof(uintptr(0)), tailSizeWords)
				}

				return chunk.start
			}
		}
	}
	return 0
}

func heapFree(ptr uintptr) {
	if ptr == 0 {
		return
	}

	index := allocedChunks.find(ptr)
	if index < 0 {
		panic("Pointer not found in allocated chunks")
	}

	freedChunks.insert(allocedChunks.chunks[index].start, allocedChunks.chunks[index].size)
	allocedChunks.remove(index)
}

func markRegion(start, end uintptr) {
	for ; start < end; start += unsafe.Sizeof(uintptr(0)) {
		p := *(*uintptr)(unsafe.Pointer(start))
		for i := 0; i < allocedChunks.count; i++ {
			chunk := allocedChunks.chunks[i]
			if chunk.start <= p && p < chunk.start+chunk.size*unsafe.Sizeof(uintptr(0)) {
				if !reachable[i] {
					reachable[i] = true
					markRegion(chunk.start, chunk.start+chunk.size*unsafe.Sizeof(uintptr(0)))
				}
			}
		}
	}
}

func heapCollect() {
	stackStart := uintptr(unsafe.Pointer(&stackBase))
	for i := range reachable {
		reachable[i] = false
	}
	markRegion(stackStart, stackBase+1)

	toFreeCount = 0
	for i := 0; i < allocedChunks.count; i++ {
		if !reachable[i] {
			if toFreeCount >= CHUNK_LIST_CAP {
				panic("To free list capacity exceeded")
			}
			toFree[toFreeCount] = unsafe.Pointer(allocedChunks.chunks[i].start)
			toFreeCount++
		}
	}

	for i := 0; i < toFreeCount; i++ {
		heapFree(uintptr(toFree[i]))
	}
}

func main() {
	stackBase = uintptr(unsafe.Pointer(&heap))

	ptr := heapAlloc(123)
	allocedChunks.dump("Allocated")
	heapFree(ptr)
	freedChunks.dump("Freed")

	heapCollect()
	freedChunks.dump("Freed after GC")
}
