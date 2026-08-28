// Package detector contains deterministic coaching candidates derived from
// safe decision context. Detector code must not reconstruct enemy knowledge
// from omniscient replay positions; it consumes the causal enemy-information
// state already exposed by timeline.TargetDeathContext.
//
// Candidate thresholds in this package are deliberately explicit and initially
// low-confidence. They require synthetic boundary tests plus real validation on
// the canonical replay and at least one contrasting match before downstream AI
// prose may treat them as trusted coaching signals.
package detector
