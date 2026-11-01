package carteiras

import (
	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/importkit"
)

// CLI é o comando cmd/importcarteiras (flags -csv e -dry-run); o main só chama
// cmdutil.Executar.
var CLI = importkit.CLI(Tag, "caminho do CSV de carteiras (default: dados/crm/carteira.csv na raiz do projeto)",
	func(csv string, dryRun bool, open cmdutil.Opener) error {
		return Run(Options{CSVFlag: csv, DryRun: dryRun}, open)
	})
