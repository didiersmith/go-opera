package dexter

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

type Candidate struct {
	DiscreteGene   uint
	ContinuousGene float64
	Fitness        float64
}

type CalcFitness func(c Candidate) float64

type Population []Candidate

func (p Population) Len() int           { return len(p) }
func (p Population) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p Population) Less(i, j int) bool { return p[i].Fitness > p[j].Fitness }

func Breed(a, b Candidate, f CalcFitness) Candidate {
	c := Candidate{}
	if rand.Intn(2) == 0 {
		c.DiscreteGene = a.DiscreteGene
	} else {
		c.DiscreteGene = b.DiscreteGene
	}
	weightA := rand.Float64()*9 + 1
	weightB := rand.Float64()*9 + 1
	mean := (a.ContinuousGene*weightA + b.ContinuousGene*weightB) / (weightA + weightB)
	c.ContinuousGene = math.Abs(rand.NormFloat64()*math.Abs(a.ContinuousGene*1.25-b.ContinuousGene) + mean)
	c.Fitness = f(c)
	return c
}

func (p Population) Print() {
	for i, c := range p {
		fmt.Printf("%d: %d / %f: %f\n", i, c.DiscreteGene, c.ContinuousGene, c.Fitness)
	}
}

func NextGeneration(p Population, popSize int, f CalcFitness) Population {
	if len(p) < 1 {
		return p
	}
	sort.Sort(p)
	if len(p) > popSize/2 {
		p = p[:popSize/2]
	}
	for len(p) < popSize {
		a := int(rand.ExpFloat64()*float64(len(p))/4) % len(p)
		b := int(rand.ExpFloat64()*float64(len(p))/4) % len(p)
		p = append(p, Breed(p[a], p[b], f))
	}
	return p
}
