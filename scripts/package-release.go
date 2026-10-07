//go:build ignore

package main

import (
 "archive/zip"
 "fmt"
 "io"
 "os"
 "path/filepath"
)

func main() {
 if len(os.Args) != 2 { panic("usage: package-release directory") }
 if err := pack(os.Args[1]); err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
}
func pack(folder string) error {
 output, err := os.Create(folder + ".zip"); if err != nil { return err }
 defer output.Close()
 writer := zip.NewWriter(output)
 entries, err := os.ReadDir(folder); if err != nil { return err }
 for _, entry := range entries {
  if entry.IsDir() { return fmt.Errorf("unexpected directory") }
  input, err := os.Open(filepath.Join(folder, entry.Name())); if err != nil { return err }
  target, err := writer.Create(entry.Name()); if err == nil { _, err = io.Copy(target, input) }
  closeErr := input.Close(); if err != nil { return err }; if closeErr != nil { return closeErr }
 }
 return writer.Close()
}
