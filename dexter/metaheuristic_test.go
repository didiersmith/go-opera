package dexter

import (
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Mock fitness function that is optimal at [5, 60]
func calcFitness(c Candidate) float64 {
	den := (math.Pow(float64(5-c.DiscreteGene), 3) + math.Abs(60-c.ContinuousGene))
	if den == 0 {
		return math.MaxFloat64
	}
	return 1 / den
}

// Test that the optimal candidate can be found within the population.
func TestEvolutionSimple(t *testing.T) {
	p := Population{
		Candidate{1, 10, 0},
		Candidate{2, 40, 0},
		Candidate{3, 60, 0},
		Candidate{4, 100, 0},
		Candidate{5, 130, 0},
		Candidate{1, 30, 0},
		Candidate{5, 80, 0},
		Candidate{2, 120, 0},
	}
	for _, c := range p {
		c.Fitness = calcFitness(c)
	}
	for i := 0; i < 10; i++ {
		p = NextGeneration(p, 40, calcFitness)
	}
	sort.Sort(p)
	winner := p[0]
	assert := assert.New(t)
	assert.Equal(uint(5), winner.DiscreteGene)
	assert.InDelta(60, winner.ContinuousGene, 1)
}

// Test that ContinuousGene can grow by a factor of 6 to the optimum.
func TestEvolutionBelow(t *testing.T) {
	p := Population{
		Candidate{1, 10, 0},
		Candidate{2, 2, 0},
		Candidate{3, 6, 0},
		Candidate{4, 8, 0},
		Candidate{5, 13, 0},
		Candidate{1, 4, 0},
		Candidate{5, 12, 0},
		Candidate{2, 7, 0},
	}
	for _, c := range p {
		c.Fitness = calcFitness(c)
	}
	for i := 0; i < 10; i++ {
		p = NextGeneration(p, 40, calcFitness)
		sort.Sort(p)
		// p.Print()
	}
	sort.Sort(p)
	winner := p[0]
	assert := assert.New(t)
	assert.Equal(uint(5), winner.DiscreteGene)
	assert.InDelta(60, winner.ContinuousGene, 1)
}
