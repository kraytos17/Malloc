package main

import (
	"fmt"
	"unsafe"
)

const (
	HEAP_CAP_WORDS   = 1024
	CHUNK_LIST_CAP   = 100
)

type Chunk struct {
	start unsafe.Pointer
	size  uintptr
}

type ChunkList struct {
	count  int
	chunks [CHUNK_LIST_CAP]Chunk
}

var (
	heap           [HEAP_CAP_WORDS]uintptr
	stackBase      uintptr
	reachableChunks [CHUNK_LIST_CAP]bool
	toFree         [CHUNK_LIST_CAP]unsafe.Pointer
	toFreeCount    int

	allocedChunks = ChunkList{}
	freedChunks   = ChunkList{
		count: 1,
		chunks: [CHUNK_LIST_CAP]Chunk{
			{start: unsafe.Pointer(&heap[0]), size: HEAP_CAP_WORDS},
		},
	}
	tmpChunks = ChunkList{}
)

func chunkListInsert(list *ChunkList, start unsafe.Pointer, size uintptr) {
	if list.count >= CHUNK_LIST_CAP {
		panic("Chunk list capacity exceeded")
	}

	list.chunks[list.count] = Chunk{start: start, size: size}

	for i := list.count; i > 0 && uintptr(list.chunks[i].start) < uintptr(list.chunks[i-1].start); i-- {
		list.chunks[i], list.chunks[i-1] = list.chunks[i-1], list.chunks[i]
	}

	list.count++
}

func chunkListMerge(dst *ChunkList, src *ChunkList) {
	dst.count = 0

	for i := 0; i < src.count; i++ {
		chunk := src.chunks[i]

		if dst.count > 0 {
			topChunk := &dst.chunks[dst.count-1]

			if uintptr(topChunk.start)+topChunk.size == uintptr(chunk.start) {
				topChunk.size += chunk.size
			} else {
				chunkListInsert(dst, chunk.start, chunk.size)
			}
		} else {
			chunkListInsert(dst, chunk.start, chunk.size)
		}
	}
}

func chunkListDump(list *ChunkList, name string) {
	fmt.Printf("%s Chunks (%d):\n", name, list.count)
	for i := 0; i < list.count; i++ {
		fmt.Printf("  start: %p, size: %d\n", list.chunks[i].start, list.chunks[i].size)
	}
}

func chunkListFind(list *ChunkList, ptr unsafe.Pointer) int {
	for i := 0; i < list.count; i++ {
		if list.chunks[i].start == ptr {
			return i
		}
	}

	return -1
}

func chunkListRemove(list *ChunkList, index int) {
	if index >= list.count {
		panic("Index out of bounds")
	}

	for i := index; i < list.count-1; i++ {
		list.chunks[i] = list.chunks[i+1]
	}

	list.count--
}

func heapAlloc(sizeBytes uintptr) unsafe.Pointer {
	sizeWords := (sizeBytes + unsafe.Sizeof(uintptr(0)) - 1) / unsafe.Sizeof(uintptr(0))

	if sizeWords > 0 {
		chunkListMerge(&tmpChunks, &freedChunks)
		freedChunks = tmpChunks

		for i := 0; i < freedChunks.count; i++ {
			chunk := freedChunks.chunks[i]
			if chunk.size >= sizeWords {
				chunkListRemove(&freedChunks, i)

				tailSizeWords := chunk.size - sizeWords
				chunkListInsert(&allocedChunks, chunk.start, sizeWords)

				if tailSizeWords > 0 {
					chunkListInsert(&freedChunks, unsafe.Pointer(uintptr(chunk.start)+sizeWords*unsafe.Sizeof(uintptr(0))), tailSizeWords)
				}

				return chunk.start
			}
		}
	}

	return nil
}

func heapFree(ptr unsafe.Pointer) {
	if ptr != nil {
		index := chunkListFind(&allocedChunks, ptr)
		if index < 0 {
			panic("Pointer not found in allocated chunks")
		}
		chunkListInsert(&freedChunks, allocedChunks.chunks[index].start, allocedChunks.chunks[index].size)
		chunkListRemove(&allocedChunks, index)
	}
}

func markRegion(start, end uintptr) {
	for ; start < end; start += unsafe.Sizeof(uintptr(0)) {
		p := *(*uintptr)(unsafe.Pointer(start))
		for i := 0; i < allocedChunks.count; i++ {
			chunk := allocedChunks.chunks[i]
			if uintptr(chunk.start) <= p && p < uintptr(chunk.start)+chunk.size*unsafe.Sizeof(uintptr(0)) {
				if !reachableChunks[i] {
					reachableChunks[i] = true
					markRegion(uintptr(chunk.start), uintptr(chunk.start)+chunk.size*unsafe.Sizeof(uintptr(0)))
				}
			}
		}
	}
}

func heapCollect() {
	stackStart := uintptr(unsafe.Pointer(&stackBase))
	for i := range reachableChunks {
		reachableChunks[i] = false
	}
	markRegion(stackStart, stackBase+1)

	toFreeCount = 0
	for i := 0; i < allocedChunks.count; i++ {
		if !reachableChunks[i] {
			if toFreeCount >= CHUNK_LIST_CAP {
				panic("To free list capacity exceeded")
			}
			toFree[toFreeCount] = allocedChunks.chunks[i].start
			toFreeCount++
		}
	}

	for i := 0; i < toFreeCount; i++ {
		heapFree(toFree[i])
	}
}

func main() {
	// Initialize stack base for the example.
	stackBase = uintptr(unsafe.Pointer(&heap))

	// Example usage:
	ptr := heapAlloc(100)
	chunkListDump(&allocedChunks, "Allocated")
	heapFree(ptr)
	chunkListDump(&freedChunks, "Freed")

	// Perform garbage collection
	heapCollect()
	chunkListDump(&freedChunks, "Freed after GC")
}
