package entities

type Pv struct {
	Cp    int
	Moves string
}

type Evaluation struct {
	Fen    string
	Depth  int
	Knodes int
	Pvs    []Pv
}
