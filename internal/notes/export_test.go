package notes

// WithCallRecorded returns a copy of g that calls recorded each time the wave has
// recorded a chunk call's outcome and freed its place in flight.
func (g *Generator) WithCallRecorded(recorded func()) *Generator {
	c := *g
	c.callRecorded = recorded
	return &c
}
