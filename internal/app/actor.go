package app

import "context"

type actorKey struct{}

// WithActor stores the caller identity for audit recording.
func WithActor(ctx context.Context, a Actor) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, actorKey{}, a)
}

// ActorFrom returns the caller stored by WithActor, or a zero Actor.
func ActorFrom(ctx context.Context) Actor {
	if ctx == nil {
		return Actor{}
	}
	a, _ := ctx.Value(actorKey{}).(Actor)
	return a
}
