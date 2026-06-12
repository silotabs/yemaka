// Package contextcore provides a small deterministic context compiler for
// assembling request, compact task state, memory, RAG, skill, tool, policy, and
// preference snippets before a prompt renderer formats them for a model.
//
// The package does no model calls and has no dependencies outside the standard
// library. It is intended to stay generic enough for agents, generated
// extensions, and capability packs to share without growing the trusted core.
package contextcore
