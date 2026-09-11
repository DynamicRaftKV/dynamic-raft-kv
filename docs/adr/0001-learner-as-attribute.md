# ADR 0001: Learner as a membership attribute

Status: Proposed; required team sign-off not yet recorded.

## Context

S1.2-05 explicitly requires this decision. The referenced SDD is unavailable.

## Decision

Use Follower, Candidate, and Leader as the only roles. Learner is a membership
attribute. A learner does not campaign or count toward a voting quorum.
No learner behavior is implemented in Stage 1.

## Consequences

Peer carries Learner separately from Role. Confirm promotion and configuration
change semantics against the SDD before the interfaces are frozen.
