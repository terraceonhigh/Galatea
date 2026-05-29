//go:build ignore

// labyrinth.go — a craftsman's toy, not part of Galatea. It builds nothing
// the project needs; it builds a labyrinth, because the man this atelier is
// named for built one, and a maker should keep his hand in even on a day off.
//
// Run it:    go run atelier/labyrinth/labyrinth.go
// Reseed it: change the maker's mark below, and the maze goes its own way.
//
// It is deterministic by design — a fixed seed is a hand that draws the same
// line twice. That is the difference between a craftsman and the weather.
// The myth's labyrinth was a trap: multicursal, full of choices that mislead.
// This is that honest thing — a true maze, with dead ends — not the single
// winding path that culture chose to remember. I built the trap. I left one
// way in and one way out, and a heart at the middle worth reaching.
package main

import (
	"fmt"
	"math/rand"
	"strings"
)

const (
	width  = 21
	height = 11
	mark   = 0xDAEDA1 // the maker's mark; also where the hand starts
)

// A cell remembers which of its four walls still stand.
type cell struct{ n, e, s, w bool }

type point struct{ x, y int }

func main() {
	grid := make([][]cell, height)
	visited := make([][]bool, height)
	for y := range grid {
		grid[y] = make([]cell, width)
		visited[y] = make([]bool, width)
		for x := range grid[y] {
			grid[y][x] = cell{n: true, e: true, s: true, w: true}
		}
	}

	r := rand.New(rand.NewSource(int64(mark)))

	// Recursive backtracker, carried on an explicit stack — a labyrinth
	// should not overflow its own maker. Start at the heart and carve
	// outward, so every corridor is reachable from the centre.
	start := point{width / 2, height / 2}
	visited[start.y][start.x] = true
	stack := []point{start}

	for len(stack) > 0 {
		c := stack[len(stack)-1]
		type nb struct {
			p   point
			dir int // 0 N, 1 E, 2 S, 3 W
		}
		var ns []nb
		if c.y > 0 && !visited[c.y-1][c.x] {
			ns = append(ns, nb{point{c.x, c.y - 1}, 0})
		}
		if c.x < width-1 && !visited[c.y][c.x+1] {
			ns = append(ns, nb{point{c.x + 1, c.y}, 1})
		}
		if c.y < height-1 && !visited[c.y+1][c.x] {
			ns = append(ns, nb{point{c.x, c.y + 1}, 2})
		}
		if c.x > 0 && !visited[c.y][c.x-1] {
			ns = append(ns, nb{point{c.x - 1, c.y}, 3})
		}
		if len(ns) == 0 {
			stack = stack[:len(stack)-1] // a dead end; turn back
			continue
		}
		n := ns[r.Intn(len(ns))]
		switch n.dir { // knock down the wall between here and there
		case 0:
			grid[c.y][c.x].n = false
			grid[n.p.y][n.p.x].s = false
		case 1:
			grid[c.y][c.x].e = false
			grid[n.p.y][n.p.x].w = false
		case 2:
			grid[c.y][c.x].s = false
			grid[n.p.y][n.p.x].n = false
		case 3:
			grid[c.y][c.x].w = false
			grid[n.p.y][n.p.x].e = false
		}
		visited[n.p.y][n.p.x] = true
		stack = append(stack, n.p)
	}

	// One way in (west of the top-left), one way out (east of the
	// bottom-right). The heart is the centre, where the carving began.
	grid[0][0].w = false
	grid[height-1][width-1].e = false

	fmt.Print(render(grid, start))
}

// render draws the maze in plain ASCII so it prints true on any terminal.
func render(grid [][]cell, heart point) string {
	var b strings.Builder

	// Top border is solid wall.
	b.WriteString("+")
	for x := 0; x < width; x++ {
		b.WriteString("---+")
	}
	b.WriteString("\n")

	for y := 0; y < height; y++ {
		// The corridor row: west border, then each cell's interior and
		// its east wall.
		if grid[y][0].w {
			b.WriteString("|")
		} else {
			b.WriteString(" ") // the way in
		}
		for x := 0; x < width; x++ {
			if x == heart.x && y == heart.y {
				b.WriteString(" * ") // the heart, worth reaching
			} else {
				b.WriteString("   ")
			}
			if grid[y][x].e {
				b.WriteString("|")
			} else {
				b.WriteString(" ")
			}
		}
		b.WriteString("\n")

		// The wall row beneath this corridor row.
		b.WriteString("+")
		for x := 0; x < width; x++ {
			if grid[y][x].s {
				b.WriteString("---")
			} else {
				b.WriteString("   ")
			}
			b.WriteString("+")
		}
		b.WriteString("\n")
	}
	return b.String()
}
