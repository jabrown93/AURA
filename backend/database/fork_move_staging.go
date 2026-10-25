package database

import (
	"context"
	"database/sql"
	"fmt"
)

// moveTarget returns the library and edition a move ends at.
func moveTarget(move MediaItemMove) (library, edition string) {
	library, edition = move.NewItem.LibraryTitle, move.NewItem.Edition
	if library == "" {
		return move.OldLibrary, move.OldEdition
	}
	if !move.Found {
		edition = move.OldEdition
	}
	return library, edition
}

// stageMediaItemMoves moves every changing source to a unique staging library and returns the moves
// rewritten to start there. Fork: applied in order, A -> B then B -> C would
// carry A's saved sets, ignored status and history on to C.
func stageMediaItemMoves(ctx context.Context, tx *sql.Tx, moves []MediaItemMove) ([]MediaItemMove, error) {
	staged := make([]MediaItemMove, len(moves))
	for i, move := range moves {
		staged[i] = move
		library, edition := moveTarget(move)
		if library == move.OldLibrary && edition == move.OldEdition {
			continue
		}
		// NUL can't appear in a media-server library title, so this never meets a real item.
		staging := fmt.Sprintf("\x00aura-move-%d", i)
		if err := moveMediaItemKey(ctx, tx, move.TMDB_ID, move.OldLibrary, move.OldEdition, staging, move.OldEdition); err != nil {
			return nil, err
		}
		// The target is unchanged: NewItem keeps it, or it is the old edition, which staging keeps.
		staged[i].OldLibrary = staging
	}
	return staged, nil
}
