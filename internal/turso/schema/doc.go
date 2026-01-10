// Package schema defines file-based JSON schemas for jj-turso storage.
//
// # Overview
//
// This package provides schemas for storing beads data as individual JSON files
// in a jj (Jujutsu) repository for 100+ agent scalability. It replaces git-based
// JSONL serialization with individual file operations that jj can merge efficiently.
//
// # Dependency Files
//
// Dependencies are stored as individual JSON files in deps/*.json with the
// filename convention: {from}--{type}--{to}.json
//
// Example: bd-abc--blocks--bd-xyz.json
//
//	{
//	  "from": "bd-abc",
//	  "to": "bd-xyz",
//	  "type": "blocks",
//	  "created_at": "2026-01-10T07:36:29Z"
//	}
//
// # Dependency Types
//
// Supported types from internal/types package:
//   - blocks - Hard dependency (issue X blocks issue Y)
//   - related - Soft relationship
//   - parent-child - Epic/subtask relationship
//   - discovered-from - Track issues discovered during work
//
// # Usage Examples
//
// Creating a dependency:
//
//	dep := &schema.DepFile{
//	    From:      "bd-abc",
//	    To:        "bd-xyz",
//	    Type:      "blocks",
//	    CreatedAt: time.Now(),
//	}
//	err := schema.WriteDepFile("deps", dep)
//
// Reading a dependency:
//
//	dep, err := schema.ReadDepFile("deps/bd-abc--blocks--bd-xyz.json")
//
// Listing all dependencies for an issue:
//
//	deps, err := schema.ListDepsForIssue("deps", "bd-abc")
//	for _, dep := range deps {
//	    fmt.Printf("%s --%s--> %s\n", dep.From, dep.Type, dep.To)
//	}
//
// Converting to/from types.Dependency:
//
//	// DepFile -> types.Dependency
//	typeDep := depFile.ToTypeDependency()
//
//	// types.Dependency -> DepFile
//	depFile := schema.FromTypeDependency(typeDep)
//
// Deleting a dependency:
//
//	err := schema.DeleteDepFile("deps", "bd-abc", "blocks", "bd-xyz")
//
// # Design Principles
//
//   - Flat JSON structure (CRDT-friendly, last-write-wins)
//   - Filename encodes relationship (enables directory listing queries)
//   - Individual files (jj merges efficiently at scale)
//   - Convertible to/from existing types.Dependency
//   - No external validation libraries (keep dependencies minimal)
//
// # See Also
//
//   - ai_docs/jj-turso.md - Full implementation plan
//   - internal/types/types.go - Original Dependency type
package schema
