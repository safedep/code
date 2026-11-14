package handlers

import (
	"os"
	"os/exec"
)

type H0 struct{}

func (H0) Handle(s string) { exec.Command(s) }

type H1 struct{}

func (H1) Handle(s string) {}

type H2 struct{}

func (H2) Handle(s string) {}

type H3 struct{}

func (H3) Handle(s string) {}

type H4 struct{}

func (H4) Handle(s string) {}

type H5 struct{}

func (H5) Handle(s string) {}

type H6 struct{}

func (H6) Handle(s string) {}

type H7 struct{}

func (H7) Handle(s string) {}

type H8 struct{}

func (H8) Handle(s string) {}

type H9 struct{}

func (H9) Handle(s string) {}

type H10 struct{}

func (H10) Handle(s string) {}

type H11 struct{}

func (H11) Handle(s string) {}

type H12 struct{}

func (H12) Handle(s string) {}

type H13 struct{}

func (H13) Handle(s string) {}

type H14 struct{}

func (H14) Handle(s string) {}

type H15 struct{}

func (H15) Handle(s string) {}

type H16 struct{}

func (H16) Handle(s string) {}

type H17 struct{}

func (H17) Handle(s string) {}

type H18 struct{}

func (H18) Handle(s string) {}

type H19 struct{}

func (H19) Handle(s string) {}

type H20 struct{}

func (H20) Handle(s string) {}

type H21 struct{}

func (H21) Handle(s string) {}

type H22 struct{}

func (H22) Handle(s string) {}

type H23 struct{}

func (H23) Handle(s string) {}

type H24 struct{}

func (H24) Handle(s string) {}

func Cleanup() { os.Remove("x") }
