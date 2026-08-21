// Command animals demonstrates a realistic nested-data pipeline composed from
// Shuttle's stream, predicate, comparator, and optional APIs.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/imbrooklyn/shuttle/comparator"
	"github.com/imbrooklyn/shuttle/predicate"
	"github.com/imbrooklyn/shuttle/stream"
)

type Animal struct {
	Name    string
	Age     int
	Habitat string
}

type AnimalSubspecies struct {
	Name    string
	Animals []Animal
}

type AnimalSpecies struct {
	Name       string
	Subspecies []AnimalSubspecies
}

type AnimalFamily struct {
	Name    string
	Species []AnimalSpecies
}

type AnimalOrder struct {
	Name     string
	Families []AnimalFamily
}

func animalsFromOrders(orders []AnimalOrder) stream.Stream[Animal] {
	// FlatMapSlice and Filter remain lazy: this function describes traversal but
	// does not inspect orders or invoke a callback yet.
	return stream.FromSlice(orders).
		FlatMapSlice(func(order AnimalOrder) []AnimalFamily { return order.Families }).
		FlatMapSlice(func(family AnimalFamily) []AnimalSpecies { return family.Species }).
		FlatMapSlice(func(species AnimalSpecies) []AnimalSubspecies { return species.Subspecies }).
		FlatMapSlice(func(subspecies AnimalSubspecies) []Animal { return subspecies.Animals })
}

func adultForestAnimals(orders []AnimalOrder) stream.Stream[Animal] {
	adult := predicate.On(
		func(animal Animal) int { return animal.Age },
		predicate.Func[int](func(age int) bool { return age >= 3 }),
	)
	inForest := predicate.On(
		func(animal Animal) string { return animal.Habitat },
		predicate.Equal("forest"),
	)
	byAgeDescendingThenName := comparator.
		ByDescending(func(animal Animal) int { return animal.Age }).
		ThenBy(func(animal Animal) string { return animal.Name })

	// SortedFunc is lazy at construction but is a finite-only barrier when this
	// returned Stream is traversed.
	return animalsFromOrders(orders).
		Filter(adult.And(inForest)).
		SortedFunc(byAgeDescendingThenName)
}

func animalNames(animals []Animal) []string {
	return stream.FromSlice(animals).
		Map(func(animal Animal) string { return animal.Name }).
		Collect()
}

func oldestAnimalName(orders []AnimalOrder) string {
	// MaxBy is a terminal and therefore consumes the complete finite input.
	return animalsFromOrders(orders).
		MaxBy(func(animal Animal) int { return animal.Age }).
		Map(func(animal Animal) string { return animal.Name }).
		OrElse("none")
}

func animalBatches(animals []Animal, size int) [][]string {
	return stream.Chunk(stream.FromSlice(animals), size).
		Map(animalNames).
		Collect()
}

func printHabitatGroups(output io.Writer, orders []AnimalOrder) {
	// GroupBy is a terminal and consumes the complete finite input. Its group
	// order is the first encounter order of each habitat.
	groups := animalsFromOrders(orders).GroupBy(func(animal Animal) string {
		return animal.Habitat
	})
	for _, group := range groups {
		fmt.Fprintf(output, "  %s: %v\n", group.Key, animalNames(group.Values))
	}
}

func run(output io.Writer) {
	orders := sampleOrders()
	eligible := adultForestAnimals(orders).Collect()

	fmt.Fprintln(output, "Eligible:", animalNames(eligible))
	fmt.Fprintln(output, "Habitats:")
	printHabitatGroups(output, orders)
	fmt.Fprintln(output, "Oldest:", oldestAnimalName(orders))
	fmt.Fprintln(output, "Batches:", animalBatches(eligible, 2))
}

func main() {
	run(os.Stdout)
}

func sampleOrders() []AnimalOrder {
	return []AnimalOrder{
		{
			Name: "Carnivora",
			Families: []AnimalFamily{
				{
					Name: "Felidae",
					Species: []AnimalSpecies{
						{
							Name: "Lion",
							Subspecies: []AnimalSubspecies{{
								Name: "Southern lion",
								Animals: []Animal{
									{Name: "Simba", Age: 6, Habitat: "savanna"},
									{Name: "Nala", Age: 4, Habitat: "savanna"},
								},
							}},
						},
						{
							Name: "Tiger",
							Subspecies: []AnimalSubspecies{{
								Name: "Bengal tiger",
								Animals: []Animal{
									{Name: "Shere Khan", Age: 8, Habitat: "forest"},
								},
							}},
						},
					},
				},
			},
		},
		{
			Name: "Primates",
			Families: []AnimalFamily{
				{
					Name: "Hominidae",
					Species: []AnimalSpecies{{
						Name: "Gorilla",
						Subspecies: []AnimalSubspecies{{
							Name: "Western lowland gorilla",
							Animals: []Animal{
								{Name: "Koko", Age: 5, Habitat: "forest"},
								{Name: "Binti", Age: 3, Habitat: "forest"},
							},
						}},
					}},
				},
				{
					Name: "Hylobatidae",
					Species: []AnimalSpecies{{
						Name: "Gibbon",
						Subspecies: []AnimalSubspecies{{
							Name: "Lar gibbon",
							Animals: []Animal{
								{Name: "Mila", Age: 2, Habitat: "forest"},
							},
						}},
					}},
				},
			},
		},
	}
}
