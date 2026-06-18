package cmd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

func writeErrorJSON(w io.Writer, workspaceRoot string, outcome *pipeline.ErrorOutcome) {
	payload := pipeline.OperationErrorPayload(workspaceRoot, outcome)
	enc := json.NewEncoder(w)
	if encErr := enc.Encode(payload); encErr != nil {
		fallback := pipeline.WriteFailedOutcome(encErr)
		_, _ = fmt.Fprintf(w, `{"error":%q,"hint":%q}`+"\n", fallback.Error, fallback.Hint)
	}
}

func writeFailedJSON(w io.Writer, workspaceRoot string, err error) {
	outcome := pipeline.WriteFailedOutcome(err)
	writeErrorJSON(w, workspaceRoot, &outcome)
}
