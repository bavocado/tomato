package adapter

// Subcommand defines a single operation an adapter can perform.
type Subcommand string

const (
	CmdCreateTask   Subcommand = "create-task"
	CmdUpdateStatus Subcommand = "update-status"
	CmdFetchTask    Subcommand = "fetch-task"
	CmdCreatePR     Subcommand = "create-pr"
	CmdUpdatePR     Subcommand = "update-pr"
	CmdCommentPR    Subcommand = "comment-pr"
	CmdMarkPRReady  Subcommand = "mark-pr-ready"
	CmdMarkPRFailed Subcommand = "mark-pr-failed"
)

// AllSubcommands lists every subcommand the protocol defines.
var AllSubcommands = []Subcommand{
	CmdCreateTask, CmdUpdateStatus, CmdFetchTask,
	CmdCreatePR, CmdUpdatePR, CmdCommentPR, CmdMarkPRReady, CmdMarkPRFailed,
}
