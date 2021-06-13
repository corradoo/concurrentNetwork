package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

var n = 5
var d = 2
var hosts = 1

type Edge struct {
	v1 int
	v2 int
}

type Graph struct {
	vertices []*Vertex
}

type Vertex struct {
	index       int
	routingChan chan []Routing
	//neighbours   []*chan []Routing
	neighbours   []*Vertex
	routingTable []Routing
	m            sync.Mutex
	packageChan  chan Package
	hosts        []*Host
	packageBuf   []Package
	bufMutex     sync.Mutex
}

type Routing struct {
	senderIndex int
	dir         int
	nextHop     int
	cost        int
	changed     bool
}

type Address struct {
	r int
	h int
}

type Package struct {
	sender   Address
	receiver Address
	routers  []int
}

type Host struct {
	id       int
	routerId int
	in       chan Package
	out      *chan Package
}

func Sender(vertex *Vertex) {
	for {
		time.Sleep(time.Millisecond * 2000)
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
				out.routingChan <- msg
			}
		}
	}
}

func Receiver(vertex *Vertex) {
	for {
		msg := <-vertex.routingChan
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

func PackageSender(vertex *Vertex) {
	for {
		var packToSend Package
		if len(vertex.packageBuf) > 0 {
			vertex.bufMutex.Lock()
			packToSend = vertex.packageBuf[0]
			vertex.bufMutex.Unlock()
		} else {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		//Check if pack is for one of our hosts and send if so
		if packToSend.receiver.r == vertex.index {
			vertex.hosts[packToSend.receiver.h].in <- packToSend

			//Remove from queue
			vertex.bufMutex.Lock()
			vertex.packageBuf = vertex.packageBuf[1:]
			vertex.bufMutex.Unlock()

			continue
		}
		vertex.m.Lock()
		for _, r := range vertex.routingTable {
			if r.dir == packToSend.receiver.r { // Find nextHop router
				if r.nextHop == -1 { // Path not defined
					vertex.m.Unlock()

					fmt.Println("\tVertex(", vertex.index, ") UNDEFINED ROUTE TO: ", r.dir)
					//Put package at the end of queue
					vertex.bufMutex.Lock()
					vertex.packageBuf = append(vertex.packageBuf, packToSend)
					vertex.packageBuf = vertex.packageBuf[1:]
					vertex.bufMutex.Unlock()
					time.Sleep(500 * time.Millisecond)

				} else { //Forward package to nextHop and delete from queue
					//Send
					sendToIndex := r.nextHop
					vertex.m.Unlock()
					for i, v := range vertex.neighbours {
						if v.index == sendToIndex {
							sendToIndex = i
							//fmt.Println("\tVertex(",vertex.index,") CHOSED:\n", sendToIndex, " next hop:",vertex.neighbours[sendToIndex].index)
							break
						}
					}
					//
					//fmt.Println("\tVertex(",vertex.index,") TRY:\n", packToSend, " next hop:",vertex.neighbours[sendToIndex].index)
					vertex.neighbours[sendToIndex].packageChan <- packToSend
					//fmt.Println("\tVertex(",vertex.index,") Package sent:\n", packToSend)

					//Remove from queue
					vertex.bufMutex.Lock()
					vertex.packageBuf = vertex.packageBuf[1:]
					vertex.bufMutex.Unlock()
				}
			}
		}
	}
}

func PackageReceiver(vertex *Vertex) {
	for {
		newPack := <-vertex.packageChan
		//fmt.Println("\tVertex(",vertex.index,") Package received:\n", newPack)
		newPack.routers = append(newPack.routers, vertex.index)

		vertex.bufMutex.Lock()
		vertex.packageBuf = append(vertex.packageBuf, newPack)
		vertex.bufMutex.Unlock()
		//fmt.Println("\tVertex(",vertex.index,")", " buffer: ", vertex.packageBuf)
	}
}

func HostRoutine(host *Host, r int, h int) {
	fmt.Println("HOST nr ", host.id, " ROUTER ", host.routerId, " TO (", r, ", ", h, ")")
	*host.out <- Package{
		receiver: Address{r: r, h: h},
		sender:   Address{r: host.routerId, h: host.id},
		routers:  make([]int, 0),
	}

	for {
		received := <-host.in
		fmt.Println("HOST(", host.routerId, ",", host.id, ") Package received:\n", received)
		newPack := Package{
			receiver: Address{r: received.sender.r, h: received.sender.h},
			sender:   Address{r: host.routerId, h: host.id},
			routers:  make([]int, 0),
		}
		*host.out <- newPack
		fmt.Println("HOST(", host.routerId, ",", host.id, ") Package sent back:\n", newPack)
		time.Sleep(time.Millisecond * 500)
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
		routingChan:  make(chan []Routing),
		neighbours:   make([]*Vertex, 0),
		routingTable: make([]Routing, 0),
		packageChan:  make(chan Package),
		hosts:        make([]*Host, 0),
	})

	//Neighbours
	newIndex := len(g.vertices) - 1
	for i := 0; i < n; i++ {
		var hop, cost int
		if i < newIndex {
			hop = newIndex - 1
			cost = newIndex - i
		} else {
			hop = newIndex + 1
			cost = i - newIndex
		}
		g.vertices[newIndex].routingTable = append(g.vertices[newIndex].routingTable, Routing{
			senderIndex: newIndex,
			cost:        cost,
			changed:     false,
			dir:         i,
			nextHop:     hop,
		})
		if i == newIndex {
			g.vertices[newIndex].routingTable[i].cost = 0
			g.vertices[newIndex].routingTable[i].nextHop = i
			g.vertices[newIndex].routingTable[i].changed = true
		}
	}

	//Hosts
	for i := 0; i < hosts; i++ {
		g.vertices[newIndex].hosts = append(g.vertices[newIndex].hosts,
			&Host{
				id:       i,
				routerId: newIndex,
				in:       make(chan Package),
				out:      &g.vertices[newIndex].packageChan,
			})
	}
	return
}

func (g *Graph) AddEdge(v1, v2 int) {
	//Chan to neighbour
	g.vertices[v1].neighbours = append(g.vertices[v1].neighbours, g.vertices[v2])
	g.vertices[v2].neighbours = append(g.vertices[v2].neighbours, g.vertices[v1])
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
	//graph.AddEdge(nodes[n-1], nodes[0])

	//Make all possible edges, and then shuffle them
	allEdges := make([]Edge, 0)
	for i := 0; i < n-1; i++ {
		for j := i + 1; j < n; j++ {
			if i != j && i+1 != j {
				allEdges = append(allEdges,
					Edge{
						v1: i,
						v2: j,
					})
			}
		}
	}
	fmt.Println(allEdges)
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(allEdges), func(i, j int) {
		allEdges[i], allEdges[j] = allEdges[j], allEdges[i]
	})
	fmt.Println(allEdges)

	//Add extra edges
	for i := 0; i < d; i++ {
		graph.AddEdge(allEdges[i].v1, allEdges[i].v2)
	}

	for i, r := range graph.vertices {
		fmt.Print("Vertex ", i, ": ")
		fmt.Println(r.routingTable)
	}

	//Routing routines
	for i := 0; i < n; i++ {
		go Sender(graph.vertices[i])
		go Receiver(graph.vertices[i])
	}

	//Forwarder routines
	for i := 0; i < n; i++ {
		go PackageSender(graph.vertices[i])
		go PackageReceiver(graph.vertices[i])
	}

	//Hosts
	for i := 0; i < n; i++ {
		for k, host := range graph.vertices[i].hosts {
			r := rand.Intn(n)
			h := rand.Intn(hosts)
			if r == host.routerId && h == host.id {
				r++
				r %= n
			}
			if k == 2 && i == 0 {
				go HostRoutine(graph.vertices[0].hosts[2], 9, 2)
			} else {
				go HostRoutine(host, r, h)
			}

		}
	}

	fmt.Println()
	<-done
}
