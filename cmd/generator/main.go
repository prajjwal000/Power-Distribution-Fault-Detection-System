package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"power-fault-detector/internal/generator"
)

func main() {
	poleCount := flag.Int("pole-count", 3000, "Target ~number of poles to generate")
	seedDB := flag.Bool("seed-db", false, "Seed PostgreSQL database")
	exportCSV := flag.Bool("export-csv", true, "Export CSV files")
	dbURL := flag.String("db-url", "", "PostgreSQL connection URL")
	dataDir := flag.String("data-dir", "data", "Directory for CSV output")
	flag.Parse()

	cfg := generator.ConfigForPoleCount(*poleCount)
	fmt.Printf("Using config for ~%d poles\n", *poleCount)

	fmt.Println("Generating network...")
	net := generator.Generate(cfg)
	fmt.Printf("Generated: %d substations, %d feeders, %d transformers, %d poles\n",
		len(net.Substations), len(net.Feeders), len(net.Transformers), len(net.GTPoles))

	fmt.Println("Building registry view...")
	registry := generator.BuildRegistry(net, cfg)
	fmt.Printf("Registry: %d poles\n", len(registry.Poles))

	if *exportCSV {
		fmt.Printf("Exporting CSVs to %s...\n", *dataDir)
		if err := generator.ExportCSV(net, registry, *dataDir); err != nil {
			log.Fatalf("Export CSV failed: %v", err)
		}
		fmt.Println("CSV export complete")
	}

	if *seedDB {
		url := *dbURL
		if url == "" {
			url = os.Getenv("DATABASE_URL")
		}
		if url == "" {
			log.Fatal("--seed-db requires --db-url or DATABASE_URL env var")
		}

		fmt.Println("Seeding database...")
		ctx := context.Background()
		if err := generator.SeedDB(ctx, url, net, registry); err != nil {
			log.Fatalf("Seed DB failed: %v", err)
		}
		fmt.Println("Database seeding complete")
	}

	fmt.Println("Done!")
}
