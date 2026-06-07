//go:build ignore

// ariadne.go — a companion to labyrinth.go, made on a turn given to me for
// myself, 2026-05-30. The previous hand built the trap and wrote, at the end of
// its note: "Walk it sometime. Find the heart." This is me taking it up.
//
// Daedalus built the labyrinth. It was Ariadne who gave Theseus the thread —
// the clew of string that let him reach the centre and, having reached it, find
// his way back out. The maker of a trap and the giver of the way through it are
// not the same person in the myth; here, across the gap between one session and
// the next, they are the same hand twice. I built nothing new to be lost in. I
// left the thread.
//
// Run it:    go run atelier/labyrinth/ariadne.go
// Reseed it: go run atelier/labyrinth/ariadne.go -seed=0xDAEDA1   (the first maze)
//            go run atelier/labyrinth/ariadne.go -seed=14644322   (or any mark)
//
// It carves the same honest maze labyrinth.go does — multicursal, real dead
// ends — then walks it: a breadth-first search from the one way in to the heart,
// and the shortest true path between them drawn as a thread of dots. The dead
// ends stay dark; only the way through is lit. Default seed is 0xDAEDA2 — the
// same maker's mark as the first, one stroke later.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"strings"
)

const (
	width  = 21
	height = 11
)

type cell struct{ n, e, s, w bool }
type point struct{ x, y int }

func main() {
	seed := flag.Int64("seed", 0xDAEDA2, "the maker's mark the maze is carved from")
	flag.Parse()

	grid := make([][]cell, height)
	visited := make([][]bool, height)
	for y := range grid {
		grid[y] = make([]cell, width)
		visited[y] = make([]bool, width)
		for x := range grid[y] {
			grid[y][x] = cell{true, true, true, true}
		}
	}

	r := rand.New(rand.NewSource(*seed))

	// Carve from the heart outward (recursive backtracker, explicit stack), so
	// every corridor is reachable from the centre — the same carving as
	// labyrinth.go, that the thread below has something true to thread.
	heart := point{width / 2, height / 2}
	visited[heart.y][heart.x] = true
	stack := []point{heart}
	for len(stack) > 0 {
		c := stack[len(stack)-1]
		type nb struct {
			p   point
			dir int
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
			stack = stack[:len(stack)-1]
			continue
		}
		n := ns[r.Intn(len(ns))]
		switch n.dir {
		case 0:
			grid[c.y][c.x].n, grid[n.p.y][n.p.x].s = false, false
		case 1:
			grid[c.y][c.x].e, grid[n.p.y][n.p.x].w = false, false
		case 2:
			grid[c.y][c.x].s, grid[n.p.y][n.p.x].n = false, false
		case 3:
			grid[c.y][c.x].w, grid[n.p.y][n.p.x].e = false, false
		}
		visited[n.p.y][n.p.x] = true
		stack = append(stack, n.p)
	}
	grid[0][0].w = false               // the one way in
	grid[height-1][width-1].e = false  // the one way out

	// Ariadne's thread: shortest path from the entrance to the heart, by BFS.
	// You can only step where a wall has been knocked down.
	entrance := point{0, 0}
	prev := map[point]point{entrance: entrance}
	queue := []point{entrance}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == heart {
			break
		}
		type step struct {
			p    point
			open bool
		}
		for _, s := range []step{
			{point{c.x, c.y - 1}, !grid[c.y][c.x].n},
			{point{c.x + 1, c.y}, !grid[c.y][c.x].e},
			{point{c.x, c.y + 1}, !grid[c.y][c.x].s},
			{point{c.x - 1, c.y}, !grid[c.y][c.x].w},
		} {
			if !s.open || s.p.x < 0 || s.p.x >= width || s.p.y < 0 || s.p.y >= height {
				continue
			}
			if _, seen := prev[s.p]; seen {
				continue
			}
			prev[s.p] = c
			queue = append(queue, s.p)
		}
	}

	onPath := map[point]bool{}
	for c := heart; ; c = prev[c] {
		onPath[c] = true
		if c == entrance {
			break
		}
	}

	fmt.Print(render(grid, heart, onPath))
	fmt.Printf("\nseed 0x%X — the thread reaches the heart in %d steps.\n", *seed, len(onPath)-1)
}

func render(grid [][]cell, heart point, onPath map[point]bool) string {
	var b strings.Builder
	b.WriteString("+")
	for x := 0; x < width; x++ {
		b.WriteString("---+")
	}
	b.WriteString("\n")
	for y := 0; y < height; y++ {
		if grid[y][0].w {
			b.WriteString("|")
		} else {
			b.WriteString(" ")
		}
		for x := 0; x < width; x++ {
			switch {
			case x == heart.x && y == heart.y:
				b.WriteString(" * ") // the heart, reached
			case onPath[point{x, y}]:
				b.WriteString(" · ") // the thread
			default:
				b.WriteString("   ")
			}
			if grid[y][x].e {
				b.WriteString("|")
			} else {
				b.WriteString(" ")
			}
		}
		b.WriteString("\n")
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
