package gol

import (
	"fmt"
	"log"
	"net/rpc"
	"strconv"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	var aliveCells []util.Cell
	for j := 0; j < p.ImageWidth; j++ {
		for i := 0; i < p.ImageHeight; i++ {
			if world[j][i] == 255 {
				aliveCells = append(aliveCells, util.Cell{i, j})
			}
		}
	}
	return aliveCells
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {

	// Create a 2D slice to store the world.
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}
	//fmt.Println("Sending ioInput command")
	c.ioCommand <- ioInput
	var filename = strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)
	//fmt.Println("Sending filename to ioFilename channel")
	c.ioFilename <- filename
	//fmt.Println("Receiving world data")
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			val, ok := <-c.ioInput
			if !ok {
				log.Fatal("Error: ioInput channel closed unexpectedly.")
			}
			world[y][x] = val
			if val == 255 {
				c.events <- CellFlipped{0, util.Cell{x, y}}
			}
		}
	}

	turn := 0
	c.events <- StateChange{turn, Executing}

	// TODO: Execute all turns of the Game of Life.
	server := "127.0.0.1:8030"
	client, _ := rpc.Dial("tcp", server)
	defer client.Close()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-ticker.C:
				request1 := stubs.Request{Grid: world, Height: p.ImageHeight, Width: p.ImageWidth, Turns: p.Turns}
				response1 := new(stubs.Response)
				fmt.Println("I have been run")
				client.Call(stubs.Reporter, request1, response1)
				c.events <- AliveCellsCount{turn, response1.Alive}
			case <-done:
				return
			}
		}
	}()

	request := stubs.Request{Grid: world, Height: p.ImageHeight, Width: p.ImageWidth, Turns: p.Turns}
	response := new(stubs.Response)
	client.Call(stubs.ProcessGameOfLife, request, response)

	fmt.Println("I have been run")
	world = response.Grid

	c.events <- StateChange{turn, Quitting}
	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: calculateAliveCells(p, world)}
	done <- true
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)

}
