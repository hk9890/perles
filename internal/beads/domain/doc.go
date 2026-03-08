// Package beads implements the domain layer for the beads issue tracking system.
//
// This package follows Domain-Driven Design (DDD) principles:
//   - Contains only pure Go code with standard library imports (no external dependencies)
//   - Defines entity types (Issue, Comment) and value objects (Status, Priority, IssueType)
//   - Implements domain logic (version comparison, minimum version checking)
//   - Has no knowledge of infrastructure concerns (file I/O, databases, CLI tools)
//
// # Core Types
//
// Issue represents a beads issue with all its fields including title, description,
// status, priority, type, labels, and dependency relationships.
//
// Comment represents a comment on an issue with author, text, and timestamp.
//
// Status, Priority, and IssueType are value objects representing the issue lifecycle
// state, urgency level, and categorization respectively.
//
// # Version Checking
//
// The package provides version comparison utilities for ensuring compatibility
// with the beads database format.
//
// # Import Aliasing
//
// Note: There is also an application beads package for service orchestration.
// When importing both packages, use aliasing to disambiguate:
//
//	import (
//	    domainbeads "github.com/hk9890/perles/internal/beads/domain"
//	    appbeads "github.com/hk9890/perles/internal/beads/application"
//	)
package domain
