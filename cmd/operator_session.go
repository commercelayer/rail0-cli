package cmd

import (
	"context"
	"errors"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/token"
)

// AN AGENT NEVER ACTS WITH AN OPERATOR'S SESSION.
//
// 'rail0 admin' refuses in agent mode, and agent mode never carries the write-mode token.
// That left the reads: with an operator's session the gateway widens the ordinary
// commands too ('payments list --scope all', 'payments get' on anyone's payment,
// 'account get --account', 'wallets --account'), and an agent on a machine where an
// operator signed in could read every merchant through them.
//
// So the session itself is what agent mode refuses, at the two places a command gets one:
//
//   - the STORED session: when the gateway said at sign-in that it is an operator's
//     (token.MarkOperator), agent mode does not carry it (bearerToken), and whatever
//     needs a session says why instead of "not logged in";
//   - a session opened ON THE SPOT, by 'auth login' or by a command signing in with its
//     -p key: when the gateway answers that it is an operator's, it is revoked at once,
//     never stored and never used.
//
// What agent mode still does with such a session stored: everything that needs none
// (health, chains, tokens, payment-methods, help), 'auth status', and 'auth logout' or
// an 'auth login' with a key that is not an operator's, which is the way out.
//
// What it cannot know: a session stored by a CLI older than the marker, one pasted with
// 'auth login --token', and a role granted after the sign-in. Each reads as not an
// operator's until the next sign-in, at most the 24 hours a session lasts.

// operatorSessionWithheld reports whether agent mode is refusing the session stored for
// this gateway because it is an operator's.
func operatorSessionWithheld(baseURL string) bool {
	return agentMode() && token.IsOperator(baseURL)
}

// operatorSessionError is the refusal, said the same way wherever it happens.
func operatorSessionError(what string) error {
	return &sessionError{
		Code: "operator_session",
		Exit: 4,
		Msg: what + " is a gateway operator's, and agent mode does not act with one: " +
			"sign in with a key whose account is not an operator's ('rail0 auth login -p @key'), " +
			"or leave this to a person at a terminal",
	}
}

func storedOperatorSessionError() error { return operatorSessionError("the stored session") }

// rememberSessionRole records, right after a session was stored, whether the gateway
// said it is an operator's. Best-effort: a marker that cannot be written leaves the
// session reading as not an operator's, which is what it read as before the marker.
func rememberSessionRole(baseURL, jwt string, operator bool) {
	if operator {
		_ = token.MarkOperator(baseURL, jwt)
		return
	}
	_ = token.UnmarkOperator(baseURL)
}

// refuseOperatorSignIn is called when a sign-in made in agent mode came back as an
// operator's: the session is revoked on the gateway (best-effort: unused, it would
// otherwise stay valid for its 24 hours), taken off the client, and refused.
func refuseOperatorSignIn(ctx context.Context, client *rail0.Client) error {
	_, _ = client.Auth.Logout(ctx)
	client.SetToken("")
	return operatorSessionError("this key's session")
}

// explainWithheldSession turns the gateway's 401 into the reason for it when agent mode
// sent the request without the stored session on purpose: "not authorized, sign in"
// would send the agent to sign in again with the same key.
func explainWithheldSession(err error) error {
	var apiErr *rail0.APIError
	if errors.As(err, &apiErr) && apiErr.Status == 401 && operatorSessionWithheld(resolveBaseURL()) {
		return storedOperatorSessionError()
	}
	return err
}
