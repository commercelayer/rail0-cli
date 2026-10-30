package cmd

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// chainRef is the state behind one command's --chain and its legacy alias --chain-id:
// both write to the same int the command reads (target), so the commands themselves
// keep working in chain ids and never see a slug.
//
// A NUMERIC value lands in target as soon as the flag is parsed: an id needs no lookup
// (and an id outside the gateway's catalog keeps working exactly as --chain-id always
// did — the gateway is the one that refuses it). A SLUG is only stored here and
// resolved by resolveChainFlags, from the root PersistentPreRunE: by then --base-url is
// parsed and checked, so the lookup goes to the gateway the command will talk to — a
// slug names a different chain on a testnet gateway than on production, and resolving
// it during flag parsing could hit the wrong one.
type chainRef struct {
	target *int
	chain  string // --chain as typed (slug or id); "" when not passed
	legacy string // --chain-id as typed; "" when not passed
}

// chainFlagValue is the pflag.Value of --chain (legacy=false) and --chain-id
// (legacy=true). Both share one chainRef, so passing both can be checked for agreement.
type chainFlagValue struct {
	ref    *chainRef
	legacy bool
}

func (v *chainFlagValue) String() string {
	if v.legacy {
		return v.ref.legacy
	}
	return v.ref.chain
}

func (v *chainFlagValue) Type() string {
	if v.legacy {
		return "int"
	}
	return "chain"
}

// Set records the value as typed. --chain-id keeps its old contract (an integer);
// --chain takes a positive id or a slug. Numeric values are applied immediately (see
// chainRef); a slug waits for resolveChainFlags.
func (v *chainFlagValue) Set(s string) error {
	s = strings.TrimSpace(s)
	if v.legacy {
		n, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("chain id must be an integer (use --chain for a slug such as arc)")
		}
		v.ref.legacy = s
		*v.ref.target = n
		return nil
	}
	if s == "" {
		return fmt.Errorf("pass a chain slug (e.g. arc) or a chain id (e.g. 5042002)")
	}
	if n, err := strconv.Atoi(s); err == nil {
		if n <= 0 {
			return fmt.Errorf("chain id must be positive (got %d)", n)
		}
		*v.ref.target = n
	} else if !chainSlugPattern(s) {
		return fmt.Errorf("%q is neither a chain id nor a chain slug (e.g. arc, base)", s)
	}
	v.ref.chain = s
	return nil
}

// chainSlugPattern accepts what the gateway publishes as a slug — lowercase letters,
// digits and dashes — case-insensitively, since ResolveChain compares that way.
func chainSlugPattern(s string) bool {
	if s == "" || len(s) > 32 {
		return false
	}
	for _, r := range strings.ToLower(s) {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

// addChainFlag registers --chain (with shorthand, "" for none) and the hidden legacy
// --chain-id on cmd, both writing the chain id into target, and wires --chain's shell
// completion to the gateway's slugs. usage describes what the chain is FOR on this
// command; the "slug or id" part is appended here so every command says it the same way.
//
// --chain-id stays accepted (hidden, no deprecation warning: scripts and the SDK docs
// still use it, and a warning on every call would land in their stderr) — so the change
// is additive. -c moves to --chain where --chain-id had it, and `-c 5042002` keeps
// meaning what it did.
func addChainFlag(cmd *cobra.Command, target *int, shorthand, usage string) {
	ref := &chainRef{target: target}
	f := cmd.Flags()
	f.VarP(&chainFlagValue{ref: ref}, "chain", shorthand,
		usage+": a slug (arc, base, …) or a chain id; the environment's gateway decides testnet vs mainnet")
	f.Var(&chainFlagValue{ref: ref, legacy: true}, "chain-id", "Legacy alias of --chain (numeric chain id)")
	_ = f.MarkHidden("chain-id")
	_ = cmd.RegisterFlagCompletionFunc("chain", completeChainSlugs)
}

// resolveChainFlags turns the --chain slugs passed to cmd into chain ids, and rejects
// a --chain and a --chain-id that name different chains. Called once per run from the
// root PersistentPreRunE (see chainRef for why not at parse time).
func resolveChainFlags(cmd *cobra.Command) error {
	var err error
	cmd.Flags().VisitAll(func(fl *pflag.Flag) {
		v, ok := fl.Value.(*chainFlagValue)
		if !ok || v.legacy || err != nil {
			return
		}
		err = v.ref.resolve(context.Background())
	})
	return err
}

// resolve applies the --chain/--chain-id pair to the target. Nothing passed leaves the
// target alone (0, so the command's own default — or "required" — applies).
func (r *chainRef) resolve(ctx context.Context) error {
	if r.chain == "" {
		return nil // --chain-id alone, or nothing: Set already wrote the target
	}
	id, err := resolveChainRef(ctx, r.chain)
	if err != nil {
		return err
	}
	if r.legacy != "" {
		if legacy, _ := strconv.Atoi(r.legacy); legacy != id {
			return newUsageError("flag_conflict",
				"--chain %s (%s) and --chain-id %s name different chains; pass one of them", r.chain, chainLabel(id), r.legacy)
		}
	}
	*r.target = id
	return nil
}

// resolveChainRef turns a slug or a numeric id into a chain id. A number is taken as is,
// with no lookup — an id the catalog does not list behaves as it always did. A slug is
// resolved against the CURRENT gateway's catalog (GET /blockchains, fetched once per
// process): a gateway serves one network class, so "arc" is Arc Testnet on staging and
// Arc on production. Unknown slugs list the available ones (rail0.ResolveChain); an
// unreachable gateway says so and points at the numeric id, which needs no lookup.
func resolveChainRef(ctx context.Context, ref string) (int, error) {
	c, err := lookupChain(ctx, ref)
	if err != nil {
		return 0, err
	}
	return c.ChainID, nil
}

// lookupChain is resolveChainRef returning the whole catalog entry. A numeric ref that
// the catalog does not list (or a catalog that cannot be fetched) yields a bare entry
// carrying just the id, so callers that only need the id behave as before; callers that
// need the entry's details (keys balance: RPC URLs, native symbol) check them.
func lookupChain(ctx context.Context, ref string) (*rail0.Blockchain, error) {
	ref = strings.TrimSpace(ref)
	if n, err := strconv.Atoi(ref); err == nil {
		if n <= 0 {
			return nil, newUsageError("invalid_chain", "chain id must be positive (got %d)", n)
		}
		chains, _ := loadChainCatalog(ctx)
		for i := range chains {
			if chains[i].ChainID == n {
				return &chains[i], nil
			}
		}
		return &rail0.Blockchain{ChainID: n}, nil
	}
	chains, err := loadChainCatalog(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve chain %q: cannot fetch the chain list from %s (pass the numeric chain id to skip the lookup): %w",
			ref, resolveBaseURL(), err)
	}
	c, err := rail0.ResolveChain(chains, ref)
	if err != nil {
		return nil, newUsageError("invalid_chain", "%s", strings.TrimPrefix(err.Error(), "rail0: "))
	}
	return c, nil
}

// configuredChain is the stored default chain: the `chain` key (slug or id, what
// `rail0 env` writes) or the legacy numeric `chain-id`. An env var outranks a config
// file value across the two keys, as it does for every single key — so a
// RAIL0_CHAIN_ID exported in CI still wins over the `chain: arc` an env switch stored.
func configuredChain() (value, origin string, ok bool) {
	v, o, ok := configuredSetting(configKeys["chain"])
	lv, lo, lok := configuredSetting(configKeys["chain-id"])
	switch {
	case ok && lok && o == "config file" && lo != "config file":
		return lv, lo, true
	case ok:
		return v, o, true
	default:
		return lv, lo, lok
	}
}

// configuredChainID resolves the stored default chain to an id. ok is false when no
// default is set; an error means one IS set but cannot be resolved (an unknown slug,
// or an unreachable gateway for a slug) — reported rather than ignored, since silently
// dropping it would turn into a "--chain is required" that hides the real cause.
func configuredChainID(ctx context.Context) (id int, origin string, ok bool, err error) {
	v, origin, ok := configuredChain()
	if !ok {
		return 0, "", false, nil
	}
	id, err = resolveChainRef(ctx, v)
	if err != nil {
		return 0, "", false, fmt.Errorf("default chain %q (%s): %w", v, origin, err)
	}
	return id, origin, true, nil
}

// errChainRequired is the shared "no --chain and no default" error.
func errChainRequired() error {
	return fmt.Errorf("--chain is required (or set a default with 'rail0 config set chain <slug|id>')")
}

// completeChainSlugs completes --chain to the current gateway's slugs, each described
// by the chain's name and id. Best-effort: an unreachable gateway completes nothing.
func completeChainSlugs(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	chains := chainCatalog()
	out := make([]string, 0, len(chains))
	for _, c := range chains {
		if c.Slug == "" {
			continue
		}
		out = append(out, c.Slug+"\t"+formatChainLabel(c.Name, c.ChainID))
	}
	sort.Strings(out)
	return out, cobra.ShellCompDirectiveNoFileComp
}
