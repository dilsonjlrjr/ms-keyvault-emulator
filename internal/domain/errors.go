package domain

import "fmt"

type ErrNotFound struct{ Kind, Name string }

func (e ErrNotFound) Error() string { return fmt.Sprintf("%s %q not found", e.Kind, e.Name) }

type ErrConflict struct{ Kind, Name string }

func (e ErrConflict) Error() string { return fmt.Sprintf("%s %q already exists", e.Kind, e.Name) }

type ErrBadParam struct{ Msg string }

func (e ErrBadParam) Error() string { return e.Msg }

type ErrForbidden struct{ Msg string }

func (e ErrForbidden) Error() string { return e.Msg }

type ErrDeleted struct{ Kind, Name string }

func (e ErrDeleted) Error() string { return fmt.Sprintf("%s %q is deleted", e.Kind, e.Name) }
