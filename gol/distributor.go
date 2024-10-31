package gol

import (
	"fmt"
	"log"
	"strconv"
	"time"
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

func makeImmutableMatrix(matrix [][]uint8) func(y, x int) uint8 {
	return func(y, x int) uint8 {
		return matrix[y][x]
	}
}

func worker(p Params, data func(y, x int) uint8, out chan<- [][]byte, startHeight, endHeight int) {
	nextState := calculateNextState(p, data, startHeight, endHeight)
	out <- nextState
}

func countAliveNeighbors(p Params, data func(y, x int) uint8, x, y int) int {
	aliveNeighbors := 0
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			if i == 0 && j == 0 {
				continue
			}
			neighborX := (x + i + p.ImageWidth) % p.ImageWidth
			neighborY := (y + j + p.ImageHeight) % p.ImageHeight
			if data(neighborY, neighborX) == 255 {
				aliveNeighbors++
			}
		}
	}
	return aliveNeighbors
}

func calculateNextState(p Params, data func(y, x int) uint8, startHeight, endHeight int) [][]byte {
	newSection := make([][]byte, endHeight-startHeight)
	for i := range newSection {
		newSection[i] = make([]byte, p.ImageWidth)
	}

	for y := startHeight; y < endHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			aliveNeighbors := countAliveNeighbors(p, data, x, y)
			if data(y, x) == 255 {
				if aliveNeighbors < 2 || aliveNeighbors > 3 {
					newSection[y-startHeight][x] = 0
				} else {
					newSection[y-startHeight][x] = 255
				}
			} else {
				if aliveNeighbors == 3 {
					newSection[y-startHeight][x] = 255
				} else {
					newSection[y-startHeight][x] = 0
				}
			}
		}
	}

	return newSection
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

func saveCurrentState(p Params, world [][]byte, c distributorChannels) {
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.Turns)
	c.ioCommand <- ioOutput
	c.ioFilename <- filename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}
	// Signal that the image output is complete
	c.events <- ImageOutputComplete{CompletedTurns: p.Turns}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {

	// TODO: Create a 2D slice to store the world.
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
		}
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	done := make(chan bool)

	turn := 0
	c.events <- StateChange{turn, Executing}

	outChannels := make([]chan [][]byte, p.Threads)
	for i := 0; i < p.Threads; i++ {
		outChannels[i] = make(chan [][]byte)
	}

	var paused = false
	var quit = false

	go func() {
		for key := range keyPresses {
			switch key {
			case 's': // Save the current state
				saveCurrentState(p, world, c)
				c.events <- ImageOutputComplete{CompletedTurns: turn}
			case 'q': // Quit the program
				quit = true
			case 'p': // Pause the program
				paused = !paused
				if paused {
					c.events <- StateChange{turn, Paused}
				} else {
					c.events <- StateChange{turn, Executing}
				}

			}
		}
	}()

	go func() {
		for {
			select {
			case <-ticker.C:
				aliveCells := calculateAliveCells(p, world)
				fmt.Printf("Alive cells: %d\n", len(aliveCells))
				c.events <- AliveCellsCount{turn, len(aliveCells)}
			case <-done:
				return
			}
		}
	}()

	// TODO: Execute all turns of the Game of Life.
	if p.Threads == 1 && !quit {
		for turn = 0; turn < p.Turns; turn++ {
			immutableData := makeImmutableMatrix(world)
			world = calculateNextState(p, immutableData, 0, p.ImageHeight)
		}
	} else {
		for turn = 0; turn < p.Turns && !quit; turn++ {
			if paused {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			immutableData := makeImmutableMatrix(world)
			newData := [][]byte{}

			rowsAThread := p.ImageHeight / p.Threads
			remainderRows := p.ImageHeight % p.Threads

			for i := 0; i < p.Threads; i++ {
				start := i * rowsAThread
				end := start + rowsAThread
				if i == p.Threads-1 { // Last thread gets the remaining rows
					end += remainderRows
				}
				go worker(p, immutableData, outChannels[i], start, end)
			}

			for i := 0; i < p.Threads; i++ {
				newData = append(newData, <-outChannels[i]...)
			}

			world = newData

		}

	}

	if quit {
		c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: calculateAliveCells(p, world)}
		saveCurrentState(p, world, c)
		c.events <- ImageOutputComplete{CompletedTurns: turn}
		c.events <- StateChange{turn, Quitting}
	}

	saveCurrentState(p, world, c)

	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: calculateAliveCells(p, world)}
	done <- true

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	defer close(c.events)
}
