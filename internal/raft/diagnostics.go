package raft

// Diagnostics is a local, advisory observation, never proof of quorum or a
// linearizable read. Counters reset on process restart; log indices persist.
type Diagnostics struct {
	ID                                                   string `json:"id"`
	Term                                                 uint64 `json:"term"`
	Leader                                               bool   `json:"leader"`
	Commit, Applied, LastIndex, SnapshotIndex            uint64
	Pending                                              int `json:"pending"`
	UncommittedEntries, UncommittedBytes                 uint64
	Accepted, Rejected, TimedOut, Elections, Leaderships uint64
	Stopped                                              bool `json:"stopped"`
}

func (n *Node) uncommittedBytesLocked() uint64 {
	var size uint64
	for i := n.commit + 1; i <= n.lastLocked(); i++ {
		size += uint64(len(n.entryLocked(i).Data)) + 64
	}
	return size
}

func (n *Node) admitLocked(size int) error {
	if len(n.pending) >= n.c.MaxPendingProposals || n.lastLocked()-n.commit >= n.c.MaxUncommittedEntries || uint64(size)+64 > n.c.MaxUncommittedBytes || n.uncommittedBytesLocked() > n.c.MaxUncommittedBytes-uint64(size)-64 {
		n.rejected++
		return ErrBusy
	}
	return nil
}

func (n *Node) Diagnostics() Diagnostics {
	n.mu.Lock()
	defer n.mu.Unlock()
	return Diagnostics{ID: n.c.ID, Term: n.hard.Term, Leader: n.role == Leader, Commit: n.commit, Applied: n.applied, LastIndex: n.lastLocked(), SnapshotIndex: n.baseLocked(), Pending: len(n.pending), UncommittedEntries: n.lastLocked() - n.commit, UncommittedBytes: n.uncommittedBytesLocked(), Accepted: n.accepted, Rejected: n.rejected, TimedOut: n.timedOut, Elections: n.elections, Leaderships: n.leaderships, Stopped: n.err != nil}
}

func (n *Node) eventLocked(event string, attrs ...any) {
	if n.c.Logger != nil {
		n.c.Logger.Info(event, append([]any{"node", n.c.ID, "term", n.hard.Term, "role", n.role, "leader", n.leader, "commit", n.commit, "applied", n.applied}, attrs...)...)
	}
}
