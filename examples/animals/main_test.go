package main

import "os"

func Example() {
	run(os.Stdout)
	// Output:
	// Eligible: [Shere Khan Koko Binti]
	// Habitats:
	//   savanna: [Simba Nala]
	//   forest: [Shere Khan Koko Binti Mila]
	// Oldest: Shere Khan
	// Batches: [[Shere Khan Koko] [Binti]]
}
