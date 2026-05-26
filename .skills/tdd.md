# Test-Driven Development (TDD) Skill

## Philosophy

**Core Principle**: Tests must verify behavior through public interfaces, not internal implementation details. Code can change entirely; tests shouldn't.

- **Good Tests (Integration/Behavioral)**: They exercise real code paths through public APIs. They describe *what* the system does, not *how* it does it. A good test reads like a specification (e.g., "syncing notes retrieves folder contents and persists new notes"). These tests survive refactorings because they do not care about internal classes, private helpers, or mocks.
- **Bad Tests (Implementation-Coupled)**: They mock internal collaborators, test private methods, or verify side-effects directly. The warning sign is when your test breaks during a refactor even though the external behavior did not change. If renaming a private method or structure field breaks your tests, the tests were coupled to the implementation rather than behavior.

---

## The Red-Green-Refactor Loop

You must implement features and fix bugs using a strict Red-Green-Refactor cycle:

1. **RED**: Write a single, failing test that describes the expected behavior via the public interface. Run the tests to verify that it fails (and fails for the correct reason).
2. **GREEN**: Write the absolute minimum implementation code required to make that specific test pass. Do not write any extra/speculative code.
3. **REFACTOR**: Clean up both the implementation and the test code (fix formatting, remove duplication, improve naming) while keeping the tests green.
4. **REPEAT**: Move on to the next vertical slice.

---

## Anti-Pattern: Horizontal Slicing

**DO NOT write all tests first, then all implementation.** This is "horizontal slicing" (treating RED as "write all tests" and GREEN as "write all code"). 

Horizontal slicing produces poor tests because:
- Tests written in bulk test *imagined* behavior, not *actual* behavior.
- You end up testing the shape of code (data structures, function signatures) rather than user-visible capabilities.
- You commit to test structures before understanding the physical constraints of the implementation.

**Correct Approach (Vertical Slicing)**: Implement features via thin "tracer bullets". One test -> one implementation -> repeat. Each test responds to what you learned from the previous cycle. Because you just wrote the code, you know exactly what behavior matters and how to verify it.
