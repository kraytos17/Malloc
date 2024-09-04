package malloc

import (
	"fmt"
	"unsafe"
)

// Constants for heap and chunk list management
const (
	HEAP_CAP_BYTES = 640_000
	HEAP_CAP_WORDS = HEAP_CAP_BYTES / int(unsafe.Sizeof(uintptr(0)))
	CHUNK_LIST_CAP = 1024
)

// Global variables for heap management
var (
	heap          [HEAP_CAP_WORDS]uintptr
	stackBase     uintptr
	reachable     [CHUNK_LIST_CAP]bool
	toFree        [CHUNK_LIST_CAP]unsafe.Pointer
	toFreeCount   int
	AllocedChunks = ChunkList{chunks: make([]Chunk, 0, CHUNK_LIST_CAP)}
	FreedChunks   = ChunkList{
		count:  1,
		chunks: []Chunk{{start: uintptr(unsafe.Pointer(&heap[0])), size: uintptr(HEAP_CAP_WORDS)}},
	}
	tmpChunks = ChunkList{chunks: make([]Chunk, 0, CHUNK_LIST_CAP)}
)

// Chunk represents a block of memory in the heap.
type Chunk struct {
	start uintptr
	size  uintptr
}

// ChunkList represents a list of memory chunks.
type ChunkList struct {
	count  int
	chunks []Chunk
}

// insert adds a new chunk to the chunk list.
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

// merge consolidates adjacent chunks in the source list into the target list.
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

// Dump prints out the list of chunks for debugging purposes.
func (list *ChunkList) Dump(name string) {
	fmt.Printf("%s Chunks (%d):\n", name, list.count)
	for i := 0; i < list.count; i++ {
		chunk := list.chunks[i]
		fmt.Printf("  start: %p, size: %d\n", unsafe.Pointer(chunk.start), chunk.size)
	}
}

// find searches for a chunk by its start address and returns its index.
func (list *ChunkList) find(ptr uintptr) int {
	for i := 0; i < list.count; i++ {
		if list.chunks[i].start == ptr {
			return i
		}
	}
	return -1
}

// remove deletes a chunk from the list by its index.
func (list *ChunkList) remove(index int) {
	if index < 0 || index >= list.count {
		panic("Index out of bounds")
	}

	copy(list.chunks[index:], list.chunks[index+1:list.count])
	list.count--
}

// HeapAlloc allocates a block of memory from the heap.
func HeapAlloc(sizeBytes uintptr) uintptr {
	sizeWords := (sizeBytes + unsafe.Sizeof(uintptr(0)) - 1) / unsafe.Sizeof(uintptr(0))

	if sizeWords > 0 {
		tmpChunks.merge(&FreedChunks)
		FreedChunks = tmpChunks

		for i := 0; i < FreedChunks.count; i++ {
			chunk := FreedChunks.chunks[i]
			if chunk.size >= sizeWords {
				FreedChunks.remove(i)

				tailSizeWords := chunk.size - sizeWords
				AllocedChunks.insert(chunk.start, sizeWords)

				if tailSizeWords > 0 {
					FreedChunks.insert(chunk.start+sizeWords*unsafe.Sizeof(uintptr(0)), tailSizeWords)
				}

				return chunk.start
			}
		}
	}
	return 0
}

// HeapFree frees a previously allocated block of memory.
func HeapFree(ptr uintptr) {
	if ptr == 0 {
		return
	}

	index := AllocedChunks.find(ptr)
	if index < 0 {
		panic("Pointer not found in allocated chunks")
	}

	FreedChunks.insert(AllocedChunks.chunks[index].start, AllocedChunks.chunks[index].size)
	AllocedChunks.remove(index)
}

// markRegion recursively marks reachable memory regions starting from a given address.
func markRegion(start, end uintptr) {
	for ; start < end; start += unsafe.Sizeof(uintptr(0)) {
		p := *(*uintptr)(unsafe.Pointer(start))
		for i := 0; i < AllocedChunks.count; i++ {
			chunk := AllocedChunks.chunks[i]
			if chunk.start <= p && p < chunk.start+chunk.size*unsafe.Sizeof(uintptr(0)) {
				if !reachable[i] {
					reachable[i] = true
					markRegion(chunk.start, chunk.start+chunk.size*unsafe.Sizeof(uintptr(0)))
				}
			}
		}
	}
}

// HeapCollect performs garbage collection by freeing unreachable memory regions.
func HeapCollect() {
	stackStart := uintptr(unsafe.Pointer(&stackBase))
	for i := range reachable {
		reachable[i] = false
	}
	markRegion(stackStart, stackBase+1)

	toFreeCount = 0
	for i := 0; i < AllocedChunks.count; i++ {
		if !reachable[i] {
			if toFreeCount >= CHUNK_LIST_CAP {
				panic("To free list capacity exceeded")
			}
			toFree[toFreeCount] = unsafe.Pointer(AllocedChunks.chunks[i].start)
			toFreeCount++
		}
	}

	for i := 0; i < toFreeCount; i++ {
		HeapFree(uintptr(toFree[i]))
	}
}

// InitHeap initializes the stack base for garbage collection.
func InitHeap() {
	stackBase = uintptr(unsafe.Pointer(&heap))
}
