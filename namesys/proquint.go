package namesys

import (
	"errors"

	context "context"

	opts "github.com/ipsn/go-ipfs/namesys/opts"
	path "github.com/ipsn/go-ipfs/path"
	proquint "github.com/bren2010/proquint"
)

type ProquintResolver struct{}

// Resolve implements Resolver.
func (r *ProquintResolver) Resolve(ctx context.Context, name string, options ...opts.ResolveOpt) (path.Path, error) {
	return resolve(ctx, r, name, opts.ProcessOpts(options), "/ipns/")
}

// resolveOnce implements resolver. Decodes the proquint string.
func (r *ProquintResolver) resolveOnce(ctx context.Context, name string, options *opts.ResolveOpts) (path.Path, error) {
	ok, err := proquint.IsProquint(name)
	if err != nil || !ok {
		return "", errors.New("not a valid proquint string")
	}
	return path.FromString(string(proquint.Decode(name))), nil
}
