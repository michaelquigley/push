package build

import (
	_ "embed"
	"fmt"

	"github.com/michaelquigley/figlet/figletlib"
	"github.com/spf13/cobra"
)

//go:embed standard.flf
var standardFont []byte

// NewVersionCmd returns a cobra command that prints a figlet banner for name
// followed by build detail.
func NewVersionCmd(name string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "show version information",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			font, err := figletlib.ReadFontFromBytes(standardFont)
			if err == nil {
				figletlib.PrintMsg(name, font, 96, font.Settings(), "left")
			}
			fmt.Println()
			fmt.Print(Detail())
			fmt.Println()
		},
	}
}
