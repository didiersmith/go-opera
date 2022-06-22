package gossip

import (
	"sort"
	"sync"
	"time"

	"github.com/Fantom-foundation/lachesis-base/hash"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/ethereum/go-ethereum/common"
	lru "github.com/hashicorp/golang-lru"
)

const (
	raceDuration time.Duration = 10 * time.Second
)

type Tournament struct {
	svc           *Service
	eventRaceChan chan *RaceEntry
	races         *lru.Cache
	sortedPeers   map[idx.ValidatorID][]string
	scores        map[idx.ValidatorID]map[string]*Score
	mu            sync.RWMutex
	sortedPeersMu sync.Mutex
}

type RaceEntry struct {
	PeerID string
	Hash   common.Hash
	T      time.Time
	Full   bool
}

type Race struct {
	start        time.Time
	participants []string
	active       bool
}

type Score struct {
	PeerID        string
	TotalScore    float64
	NumRaces      float64
	TotalPosition float64
}

type ScoreList []*Score

func (s ScoreList) Len() int { return len(s) }
func (s ScoreList) Less(i, j int) bool {
	return s[i].TotalScore > s[j].TotalScore
}
func (s ScoreList) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func NewTournament(svc *Service) *Tournament {
	t := &Tournament{
		svc:           svc,
		eventRaceChan: make(chan *RaceEntry, 8192),
		sortedPeers:   make(map[idx.ValidatorID][]string),
		scores:        make(map[idx.ValidatorID]map[string]*Score),
	}
	t.races, _ = lru.New(4096)
	go t.watchRaces()
	go t.finishRaces()
	return t
}

func (t *Tournament) watchRaces() {
	for {
		e := <-t.eventRaceChan
		t.mu.Lock()
		var race *Race
		raceI, ok := t.races.Get(e.Hash)
		if ok {
			race = raceI.(*Race)
		} else {
			race = &Race{
				start:  time.Now(),
				active: true,
			}
			t.races.Add(e.Hash, race)
		}
		t.mu.Unlock()
		if time.Now().Sub(race.start) > raceDuration {
			continue
		}
		race.participants = append(race.participants, e.PeerID)
	}
}

func (t *Tournament) finishRaces() {
	for {
		time.Sleep(raceDuration)
		t.mu.RLock()
		for _, k := range t.races.Keys() {
			raceI, ok := t.races.Get(k)
			if !ok {
				continue
			}
			race := raceI.(*Race)
			if !race.active || time.Now().Sub(race.start) < raceDuration || len(race.participants) < 10 {
				continue
			}
			e := t.svc.store.GetEventPayload((hash.Event)(k.(common.Hash)))
			if e == nil {
				continue
			}
			// log.Info("Settling race for event", "event", e, "validator", e.Locator().Creator)
			scores, ok := t.scores[e.Locator().Creator]
			if !ok {
				scores = make(map[string]*Score)
				t.scores[e.Locator().Creator] = scores
			}
			for i, pid := range race.participants {
				pScore, ok := scores[pid]
				if !ok {
					pScore = &Score{pid, 0, 0, 0}
					scores[pid] = pScore
				}
				pScore.TotalScore += float64(len(pid)-i) / float64(len(pid))
				pScore.TotalPosition += float64(i)
				pScore.NumRaces++
			}
			race.active = false
		}
		t.mu.RUnlock()
		for vid, scoreMap := range t.scores {
			if len(scoreMap) == 0 {
				continue
			}
			scoreList := make(ScoreList, 0, len(scoreMap))
			for _, score := range scoreMap {
				scoreList = append(scoreList, score)
			}
			sort.Sort(scoreList)
			// winner := scoreList[0]
			// loser := scoreList[len(scoreList)-1]
			// log.Info("Sorted peers", "validator", vid,
			// 	"winner", winner.PeerID,
			// 	"pos", winner.TotalPosition,
			// 	"num", winner.NumRaces,
			// 	"score", winner.TotalScore,
			// 	"loser", loser.PeerID,
			// 	"pos", loser.TotalPosition,
			// 	"num", loser.NumRaces,
			// 	"score", loser.TotalScore,
			// 	"len", len(scoreList),
			// )
			peers := make([]string, len(scoreList))
			for i, s := range scoreList {
				peers[i] = s.PeerID
				s.TotalScore *= 0.9
			}
			t.sortedPeersMu.Lock()
			t.sortedPeers[vid] = peers
			t.sortedPeersMu.Unlock()
		}
	}
}

func (t *Tournament) GetSortedPeers(vid idx.ValidatorID) []string {
	t.sortedPeersMu.Lock()
	peers, _ := t.sortedPeers[vid]
	t.sortedPeersMu.Unlock()
	return peers
}
