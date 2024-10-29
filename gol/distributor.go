package gol

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

func countAliveNeighbors(p Params, world [][]byte, x, y int) int {
	aliveNeighbors := 0
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			if i == 0 && j == 0 {
				continue
			}
			neighborX := (x + i + p.ImageWidth) % p.ImageWidth
			neighborY := (y + j + p.ImageHeight) % p.ImageHeight
			if world[neighborY][neighborX] == 255 {
				aliveNeighbors++
			}
		}
	}
	return aliveNeighbors
}

func calculateNextState(p Params, world [][]byte) [][]byte {
	newWorld := make([][]byte, p.ImageHeight)
	for i := range newWorld {
		newWorld[i] = make([]byte, p.ImageWidth)
	}

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			aliveNeighbors := countAliveNeighbors(p, world, x, y)
			if world[y][x] == 255 {
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
func distributor(p Params, c distributorChannels) {

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

	turn := 0
	c.events <- StateChange{turn, Executing}

	// TODO: Execute all turns of the Game of Life.
	for turn = 0; turn < p.Turns; turn++ {
		world = calculateNextState(p, world)
	}

	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: calculateAliveCells(p, world)}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
