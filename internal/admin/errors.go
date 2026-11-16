package admin

import "errors"

var ErrUsernameExists = errors.New("username sudah digunakan")

var ErrActorNotAvailable = errors.New("data belum terdaftar atau sudah punya akun")
