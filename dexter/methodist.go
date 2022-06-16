package dexter

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

type Method [4]byte

type MethodEventType int

const (
	UpdatedReserves MethodEventType = iota
	Confirmed                       = iota
	Pending                         = iota
	Fired                           = iota
	Success                         = iota

	NumMethodEventTypes = iota
)

var (
	blacklist = map[Method]struct{}{
		[4]byte{79, 139, 111, 91}:   struct{}{}, // Trader who fails a ton of txs
		[4]byte{0, 0, 0, 0}:         struct{}{}, // Arbitrageur
		[4]byte{162, 55, 249, 246}:  struct{}{}, // rebalance
		[4]byte{42, 48, 52, 139}:    struct{}{}, // earnMany(uint256[] _pids) -- tiny txs
		[4]byte{253, 119, 145, 205}: struct{}{}, // many tiny txs
		[4]byte{245, 25, 111, 20}:   struct{}{}, // swapLinear
	}
)

type MethodEvent struct {
	M      Method
	Type   MethodEventType
	TxHash common.Hash
}

type Methodist struct {
	stats         map[Method][]int
	txHashes      map[Method]common.Hash
	events        chan MethodEvent
	root          string
	flushInterval time.Duration
	mu            sync.Mutex
}

func NewMethodist(root string, flushInterval time.Duration) *Methodist {
	m := &Methodist{
		stats:         make(map[Method][]int),
		txHashes:      make(map[Method]common.Hash),
		events:        make(chan MethodEvent, 256),
		root:          root,
		flushInterval: flushInterval,
	}
	m.LoadData()
	go m.Run()
	return m
}

func (m *Methodist) Record(e MethodEvent) {
	_, blacklisted := blacklist[e.M]
	if blacklisted {
		return
	}
	select {
	case m.events <- e:
	default:
	}
}

func (m *Methodist) Interested(method Method) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, blacklisted := blacklist[method]
	if blacklisted {
		return false
	}
	_, ok := m.stats[method]
	return ok
}

func (m *Methodist) LoadData() {
	file, err := os.Open(m.root + "/data/methods.csv")
	if err != nil {
		log.Error("Could not open method file", "err", err)
		return
	}
	r := csv.NewReader(file)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Error("Error reading method file", "err", err)
		}
		mStr := record[0]
		if mStr == "Method" {
			continue
		}
		mParts := strings.Split(mStr[1:len(mStr)-1], " ")
		method := Method{
			Atob(mParts[0]),
			Atob(mParts[1]),
			Atob(mParts[2]),
			Atob(mParts[3]),
		}
		_, blacklisted := blacklist[method]
		if blacklisted {
			continue
		}
		m.stats[method] = make([]int, NumMethodEventTypes)
		for t := 0; t < NumMethodEventTypes; t++ {
			m.stats[method][t] = Atoi(record[t+1])
		}
		m.txHashes[method] = common.HexToHash(record[len(record)-1])
	}
}

func (m *Methodist) Run() {
	interval := time.After(m.flushInterval)
	for {
		select {
		case e := <-m.events:
			m.mu.Lock()
			if _, ok := m.stats[e.M]; ok {
				m.stats[e.M][e.Type]++
			} else {
				m.stats[e.M] = make([]int, NumMethodEventTypes)
				m.stats[e.M][e.Type]++
				m.txHashes[e.M] = e.TxHash
			}
			m.mu.Unlock()
		case <-interval:
			m.mu.Lock()
			file, err := os.OpenFile(m.root+"/data/methods.csv", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
			if err != nil {
				log.Error("Could not open method file", "err", err)
				continue
			}
			fmt.Fprint(file, "Method,UpdatedReserves,Confirmed,Pending,Fired,Success,Example\n")
			l := m.Sort()
			for _, ms := range l {
				fmt.Fprintf(file, "%v,%d,%d,%d,%d,%d,%s\n", ms.M, ms.Stats[UpdatedReserves], ms.Stats[Confirmed], ms.Stats[Pending], ms.Stats[Fired], ms.Stats[Success], m.txHashes[ms.M].Hex())
			}
			m.mu.Unlock()
			file.Close()
			interval = time.After(m.flushInterval)
		}
	}
}

type MethodStats struct {
	M     Method
	Stats []int
}
type MethodStatsList []MethodStats

func (s *MethodStats) Score() float64 {
	if s.Stats[Pending] > 0 {
		if s.Stats[Success] > 0 {
			return float64(s.Stats[Success]) / float64(s.Stats[Pending])
		} else {
			return 0.1 * float64(s.Stats[Fired]) / float64(s.Stats[Pending])
		}
	}
	if s.Stats[Confirmed] == 0 {
		return 0
	}
	return float64(s.Stats[UpdatedReserves]) / float64(s.Stats[Confirmed])
}

func (p MethodStatsList) Len() int      { return len(p) }
func (p MethodStatsList) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p MethodStatsList) Less(i, j int) bool {
	return p[i].Score() > p[j].Score()
}

func (m *Methodist) Sort() MethodStatsList {
	l := make(MethodStatsList, len(m.stats))
	i := 0
	for method, stats := range m.stats {
		l[i] = MethodStats{method, stats}
		i++
	}
	sort.Sort(l)
	return l
}

func (m *Methodist) GetLists() (white []Method, black []Method) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for method, stats := range m.stats {
		if stats[Fired] > 20 {
			if float64(stats[Success])/float64(stats[Fired]) < 0.08 {
				black = append(black, method)
				// fmt.Printf("Blacklisting due to low success rate, %v, %d/%d, %v\n", method, stats[Success], stats[Fired], stats)
			} else {
				white = append(white, method)
			}
			continue
		}
		if stats[Pending] > 1000 {
			if float64(stats[Fired])/float64(stats[Pending]) < 0.005 {
				// fmt.Printf("Blacklisting due to low firing rate, %v, %d/%d, %v\n", method, stats[Fired], stats[Pending], stats)
				black = append(black, method)
			} else {
				white = append(white, method)
			}
			continue
		}
		if stats[Confirmed] < 20 || float64(stats[UpdatedReserves])/float64(stats[Confirmed]) < 0.4 {
			// fmt.Printf("Blacklisting due to low update rate, %v, %d/%d, %v\n", method, stats[UpdatedReserves], stats[Confirmed], stats)
			black = append(black, method)
		} else {
			white = append(white, method)
		}
	}
	return white, black
}
