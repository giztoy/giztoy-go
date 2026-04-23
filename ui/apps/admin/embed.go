package adminui

import (
	"embed"
	"io/fs"
)

//go:generate npm exec --prefix ../.. --package @tailwindcss/cli -- tailwindcss -i ./styles.css -o ./dist/app.css --minify
//go:generate npm exec --prefix ../.. --package esbuild -- esbuild index.html app.tsx --bundle --format=esm --platform=browser --target=esnext --jsx=automatic --loader:.html=copy --outdir=dist --entry-names=[name] --minify

// distFS holds generated admin UI assets.
//
//go:embed dist/*
var distFS embed.FS

func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
