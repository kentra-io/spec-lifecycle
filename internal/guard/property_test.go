package guard

import (
	"fmt"
	"math/rand"
	"testing"
)

// TestRun_FromEmptyReplayMatchesLiveAcrossRandomHistories is the
// property/determinism test implementation-plan.md §9 asks for: "from-empty
// replay == live projection across 100 histories". It generates
// numHistories independent, randomized-but-seeded change histories —
// each one an actual sequence of ADD/MODIFY/REMOVE/RENAME archive steps,
// built through internal/spec's own Fold (fixtureBuilder.step in
// guard_test.go, the SAME code path a real `lifecycle archive` uses) — and
// asserts guard.Run reports every one of them clean. Since fixtureBuilder
// re-derives the live spec.md via the SAME Fold call used to author each
// archived delta, a non-clean result here means guard itself has a false
// positive, not that the fixture is wrong.
//
// # Generator design
//
// One math/rand source, seeded with a fixed constant (propertyTestSeed) so
// the whole suite is 100% reproducible across machines and CI runs — no
// -update, no golden files, just a deterministic pseudo-random stream
// consumed in a fixed order (numHistories outer iterations, each
// generating its own capability set then step sequence). Each history:
//
//   - picks 1-3 capability names from a small fixed pool;
//   - performs 5-15 steps, each against one randomly chosen capability;
//   - a capability with no live requirements yet is always ADDed to first
//     (MODIFIED/REMOVED/RENAMED all require an existing target — asking
//     Fold to error on a random empty-capability op would be testing
//     Fold's error paths, already covered by fold_test.go, not this
//     property);
//   - otherwise picks uniformly among ADD/MODIFY/REMOVE/RENAME, weighted
//     so the requirement set tends to grow (ADD most likely) rather than
//     often collapsing back to empty, which would make most steps forced
//     ADDs and understate the other three ops' coverage.
//
// Every ADDed or RENAMED-to name is drawn from a single process-wide
// monotonic counter (nextFreshName), so it can never collide with any
// existing requirement in ANY capability, in ANY history — the generator
// itself never needs to trigger (and therefore never accidentally
// triggers) one of Fold's own conflict errors.
//
// Kept intentionally to a single capability's steps per "step" (not a
// multi-capability change touching several capabilities in one archive):
// this covers "many interleaved single-capability histories" fully — the
// DISTINCT "one change folds two capabilities at once" shape is instead
// covered by the small, explicit, easier-to-read
// TestRun_CleanMultiChangeMultiCapability in guard_test.go.
const (
	propertyTestSeed     = 20260704
	propertyNumHistories = 100
)

func TestRun_FromEmptyReplayMatchesLiveAcrossRandomHistories(t *testing.T) {
	rng := rand.New(rand.NewSource(propertyTestSeed))
	var freshCounter int
	freshName := func() string {
		freshCounter++
		return fmt.Sprintf("Req-%04d", freshCounter)
	}

	capPool := []string{"auth", "billing", "search", "notifications"}

	for h := 0; h < propertyNumHistories; h++ {
		t.Run(fmt.Sprintf("history-%02d", h), func(t *testing.T) {
			b := newFixtureBuilder(t)

			numCaps := 1 + rng.Intn(3) // 1-3 capabilities
			caps := make([]string, numCaps)
			copy(caps, capPool[:numCaps])
			live := map[string][]string{}
			for _, c := range caps {
				live[c] = nil
			}

			steps := 5 + rng.Intn(11) // 5-15 steps
			for s := 0; s < steps; s++ {
				capability := caps[rng.Intn(len(caps))]
				names := live[capability]

				op := "ADD"
				if len(names) > 0 {
					switch roll := rng.Intn(100); {
					case roll < 40:
						op = "ADD"
					case roll < 65:
						op = "MODIFY"
					case roll < 80:
						op = "REMOVE"
					default:
						op = "RENAME"
					}
				}

				switch op {
				case "ADD":
					name := freshName()
					b.step(capability, addDeltaText(name))
					live[capability] = append(live[capability], name)
				case "MODIFY":
					name := names[rng.Intn(len(names))]
					b.step(capability, modifyDeltaText(name, fmt.Sprintf(" (rev %d)", s)))
				case "REMOVE":
					idx := rng.Intn(len(names))
					name := names[idx]
					b.step(capability, removeDeltaText(name))
					live[capability] = append(names[:idx], names[idx+1:]...)
				case "RENAME":
					idx := rng.Intn(len(names))
					from := names[idx]
					to := freshName()
					b.step(capability, renameDeltaText(from, to))
					names[idx] = to
					live[capability] = names
				}
			}

			res, err := Run(Options{Root: b.root})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if !res.Summary.Clean {
				t.Fatalf("history-%02d: from-empty replay did not match the live projection: %+v", h, res.Findings)
			}
		})
	}
}
