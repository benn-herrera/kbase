package survey

// WordCap bounds every string the artifact copies out of a document. A gist
// is a routing hint — enough for the taxonomy stage to tell a section about
// installation from one about scripting — and every one of them is paid for
// in the whole-corpus artifact that stage reads, so the cap is deliberately
// short.
//
// It is the argument every caller hands text.CapWords: the capping RULE is
// neutral and lives there, this NUMBER is the artifact's own promise and
// lives here. Adapters cap with it; nobody re-implements the capper, and
// nobody picks a different number — a second adapter that capped differently
// would produce an artifact with different rules under the same schema id.
//
// Titles run through it as well as gists. A title is paid for in the same
// whole-corpus artifact and is bounded by nothing in the source — a Markdown
// setext heading's title is the entire paragraph above the underline, so one
// 300-word paragraph would otherwise buy a 300-word title. Truncating is safe
// because a title is a label; the offsets it labels are untouched.
const WordCap = 40
