package seedusers

import "github.com/rotaperfumes/shared/cmdutil"

// CLI é o comando cmd/seedusers (flags -no-exec, -dry-run e -show-password);
// o main só chama cmdutil.Executar. -dry-run e -no-exec não tocam o banco e
// por isso não exigem DB_USUARIO/DB_SENHA (SEC-10, M-03); a execução real
// usa o config.Load completo (SEC-11).
func CLI(args []string, amb cmdutil.Ambiente) error {
	var opts Options
	fs := cmdutil.NovasFlags(Tag, amb)
	fs.BoolVar(&opts.NoExec, "no-exec", false, "apenas grava os SQLs com hash em tmp/seed (mantidos; apague depois); não executa no MariaDB")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "imprime hashes gerados sem gravar arquivos (hash completo só com -show-password)")
	fs.BoolVar(&opts.ShowPassword, "show-password", false, "exibe as senhas de seed geradas/usadas e os hashes completos no console (cuidado: evite em ambientes compartilhados)")
	if err := cmdutil.LerFlags(fs, args); err != nil {
		return err
	}
	cfg, err := cmdutil.Configurar(amb, opts.DryRun || opts.NoExec)
	if err != nil {
		return err
	}
	return Run(cfg, opts, Deps{Out: amb.Saida, ProjectRoot: cmdutil.FindProjectRoot, RunSQL: RunMySQL})
}
