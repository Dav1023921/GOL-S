package stubs

var ProcessGameOfLife = "GolOperations.ProcessAllTurns"
var Reporter = "GolOperations.CalculateAliveCells"

type Response struct {
	Grid  [][]byte
	Alive int
}

type Request struct {
	Grid   [][]byte
	Width  int
	Height int
	Turns  int
}
