# ADR 0002: Draft interface contracts and Stage 1 scaffolding

Status: Proposed; NOT FROZEN.

## Context

The owner approved beginning Stage 1 after analysis identified missing concrete
types, persistence guarantees, inbound RPC dispatch, and API operation semantics.
The referenced SDD has not been supplied. See ../contracts.md for open decisions.

## Decision

Preserve the seven packages, raft-owned Transport, storage-owned Storage,
state-owned StateMachine, and two binaries. Move command/result type declarations
into Stage 1; codecs and KV business logic remain Stage 2. Use opaque bytes across
the Raft/state boundary in this draft. Define plain Go RPC types and generated
protobuf types separately; future adapters map them. Assert the handwritten
transport wrapper satisfies Transport, not the generated gRPC client.

Stage 1 permits listener lifecycle, configuration, JSON parsing, CLI HTTP calls,
generated code, and explicit 501/Unimplemented responses. It implements no Raft,
KV, WAL, membership, authentication, or fault injection logic.

Bearer-token authentication remains out of MVP scope for now, per S1.4-05.

## Consequences

Generated contracts and compile-time mocks are review aids, not proof of semantic
sufficiency. SDD-dependent choices remain drafts and do not unlock Stage 2/3.
Freeze requires documented review and resolution of the open decisions.
