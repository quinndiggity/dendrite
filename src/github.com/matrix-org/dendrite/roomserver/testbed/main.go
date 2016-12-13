package main

import (
	"bufio"
	"github.com/matrix-org/dendrite/roomserver/api"
	"github.com/matrix-org/dendrite/roomserver/input"
	"github.com/matrix-org/dendrite/roomserver/storage"
	"log"
	"os"
	"strings"
)

func main() {
	var db storage.Database
	if err := db.Open(os.Args[1]); err != nil {
		log.Fatal("Error opening database", err)
	}

	file, err := os.Open(os.Args[2])
	if err != nil {
		log.Fatal("Error opening file", err)
	}
	defer file.Close()

	kinds := map[string]int{
		"O": api.KindOutlier,
		"B": api.KindBackfill,
		"J": api.KindJoin,
		"N": api.KindNew,
	}

	handler := input.InputEventHandler{}
	handler.Setup(&db)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			log.Fatalf("Line %q the wrong number of parts", line)
		}
		var inputEvent api.InputEvent
		inputEvent.Kind = kinds[parts[0]]
		inputEvent.Event = []byte(parts[1])
		if parts[2] == "" {
			inputEvent.State = nil
		} else {
			inputEvent.State = strings.Split(parts[2], ",")
		}
		if err = handler.Handle(&inputEvent); err != nil {
			log.Fatal(err)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}
