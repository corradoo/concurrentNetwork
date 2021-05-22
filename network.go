package main

import (
	"fmt"
	"math"
	"sync"
	"time"
)

var n = 6

type Graph struct {
	vertices []*Vertex
}

type Vertex struct {
	index        int
	in           chan []Routing
	neighbours   []*chan []Routing
	routingTable []Routing
	m            sync.Mutex
}

type Routing struct {
	senderIndex int
	dir         int
	nextHop     int
	cost        int
	changed     bool
}

func Sender(vertex *Vertex) {
	for {
		time.Sleep(time.Millisecond * 500)
		msg := make([]Routing, 0)
		//crit section
		vertex.m.Lock()
		for i, r := range vertex.routingTable {
			if r.changed == true {
				msg = append(msg, r)
				vertex.routingTable[i].changed = false
			}
		}
		vertex.m.Unlock()

		if len(msg) > 0 {
			fmt.Println("Vertex ", vertex.index, " sending: ", msg)
			for _, out := range vertex.neighbours {
				*out <- msg
			}
		}
	}
}

func Receiver(vertex *Vertex) {
	for {
		msg := <-vertex.in
		fmt.Println("Vertex ", vertex.index, "received: ", msg)

		vertex.m.Lock()
		//Comp to curr cost
		for _, r := range msg {
			index := r.dir
			newCost := r.cost + 1
			if newCost < vertex.routingTable[index].cost {
				vertex.routingTable[index].cost = newCost
				vertex.routingTable[index].nextHop = r.senderIndex
				vertex.routingTable[index].changed = true
			}
		}
		vertex.m.Unlock()
		fmt.Println("Vertex ", vertex.index, "state: ", vertex.routingTable)
	}
}
func New() *Graph {
	return &Graph{
		vertices: []*Vertex{},
	}
}

func (g *Graph) AddNode() (id int) {
	id = len(g.vertices)
	g.vertices = append(g.vertices, &Vertex{
		index:        id,
		in:           make(chan []Routing),
		neighbours:   make([]*chan []Routing, 0),
		routingTable: make([]Routing, 0),
	})
	newIndex := len(g.vertices) - 1
	for i := 0; i < n; i++ {
		g.vertices[newIndex].routingTable = append(g.vertices[newIndex].routingTable, Routing{
			senderIndex: newIndex,
			cost:        math.MaxInt32,
			changed:     false,
			dir:         i,
			nextHop:     -1,
		})
		if i == newIndex {
			g.vertices[newIndex].routingTable[i].cost = 0
			g.vertices[newIndex].routingTable[i].nextHop = i
			g.vertices[newIndex].routingTable[i].changed = true
		}
	}
	/*for i, r := range g.vertices[newIndex].routingTable {
		r.senderIndex = newIndex
		r.cost = math.MaxInt32
		r.changed = false
		r.dir = i
		r.nextHop = -1
		if i == newIndex {
			r.cost = 0
			r.nextHop = newIndex
			r.changed = true
		}
	}*/
	return
}

func (g *Graph) AddEdge(v1, v2 int) {
	//Chan to neighbour
	g.vertices[v1].neighbours = append(g.vertices[v1].neighbours, &g.vertices[v2].in)
	g.vertices[v2].neighbours = append(g.vertices[v2].neighbours, &g.vertices[v1].in)

	//Routing table info for both vertices
	/*g.vertices[v1].routingTable[v2].dir = v2
	g.vertices[v1].routingTable[v2].nextHop = v2
	g.vertices[v1].routingTable[v2].cost = 1
	g.vertices[v1].routingTable[v2].senderIndex = v1
	g.vertices[v1].routingTable[v2].changed = true

	g.vertices[v2].routingTable[v1].nextHop = v1
	g.vertices[v2].routingTable[v1].dir = v1
	g.vertices[v2].routingTable[v1].cost = 1
	g.vertices[v2].routingTable[v1].senderIndex = v2
	g.vertices[v2].routingTable[v1].changed = true*/
}

func (g *Graph) Nodes() []int {
	vertices := make([]int, len(g.vertices))
	for i := range g.vertices {
		vertices[i] = i
	}
	return vertices
}

func main() {

	graph := New()

	nodes := make([]int, n)

	done := make(chan bool)
	//Make nodes
	for i := 0; i < n; i++ {
		nodes[i] = graph.AddNode()
	}

	//Put 'normal' edges
	for i := 0; i < n-1; i++ {
		graph.AddEdge(nodes[i], nodes[i+1])
	}
	graph.AddEdge(nodes[n-1], nodes[0])

	for i, r := range graph.vertices {
		fmt.Print("Vertex ", i, ": ")
		fmt.Println(r.routingTable)
	}

	for i := 0; i < n; i++ {
		go Sender(graph.vertices[i])
		go Receiver(graph.vertices[i])
	}
	fmt.Println()
	<-done
}
