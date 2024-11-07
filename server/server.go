package main

import (
	"flag"
	"net"
	"net/rpc"
	"sync"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

// helper functions
func makeImmutableMatrix(matrix [][]uint8) func(y, x int) uint8 {
	return func(y, x int) uint8 {
		return matrix[y][x]
	}
}

func countAliveNeighbors(width, height, turns int, data func(y, x int) uint8, x, y int) int {
	aliveNeighbors := 0
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			if i == 0 && j == 0 {
				continue
			}
			neighborX := (x + i + width) % width
			neighborY := (y + j + height) % height
			if data(neighborY, neighborX) == 255 {
				aliveNeighbors++
			}
		}
	}
	return aliveNeighbors
}

func calculateNextState(width, height, turns int, data func(y, x int) uint8) [][]byte {
	newWorld := make([][]byte, height)
	for i := range newWorld {
		newWorld[i] = make([]byte, width)
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			aliveNeighbors := countAliveNeighbors(width, height, turns, data, x, y)
			if data(y, x) == 255 {
				if aliveNeighbors < 2 || aliveNeighbors > 3 {
					newWorld[y][x] = 0
				} else {
					newWorld[y][x] = 255
				}
			} else {
				if aliveNeighbors == 3 {
					newWorld[y][x] = 255
				} else {
					newWorld[y][x] = 0
				}
			}
		}
	}

	return newWorld
}

var mu sync.Mutex

// main operation
func process(width, height, turns int, world *[][]byte) [][]byte {
	turn := 0
	for turn = 0; turn < turns; turn++ {
		immutableData := makeImmutableMatrix(*world)
		mu.Lock()
		nextWorld := calculateNextState(width, height, turns, immutableData)
		*world = nextWorld
		mu.Unlock()

	}
	return *world
}

func calculateAliveCells(width, height int, world [][]byte) []util.Cell {
	var aliveCells []util.Cell
	for j := 0; j < height; j++ {
		for i := 0; i < width; i++ {
			if world[j][i] == 255 {
				aliveCells = append(aliveCells, util.Cell{i, j})
			}
		}
	}
	return aliveCells
}

type GolOperations struct{}

var world [][]byte

func (g *GolOperations) ProcessAllTurns(req stubs.Request, res *stubs.Response) (err error) {
	world = req.Grid
	res.Grid = process(req.Width, req.Height, req.Turns, &world)
	return
}

func (g *GolOperations) CalculateAliveCells(req stubs.Request, res *stubs.Response) (err error) {
	mu.Lock()
	res.Alive = len(calculateAliveCells(req.Width, req.Height, world))
	mu.Unlock()
	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&GolOperations{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
