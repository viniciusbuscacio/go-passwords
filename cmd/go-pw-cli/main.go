// go-pw-cli is the command-line interface of go-passwords: full control of a
// vault from scripts, servers and AI agents, no GUI required.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/viniciusbuscacio/go-passwords/internal/vault"
	"golang.org/x/term"
)

// appVersion is stamped by the release workflow via -ldflags.
var appVersion = "dev"

const actor = "cli"

const usage = `go-pw-cli — go-passwords command line

Usage:
  go-pw-cli <command> [args] [flags]

Commands:
  init [path]                 create a new vault
  list [-q term] [--json]     list secrets (passwords omitted)
  get <title|id> [--field f] [--json]
  set <title> [--username u] [--password p] [--url u] [--notes n] [--category id]
  delete <title|id>
  generate [--length n] [--no-symbols]
  categories list|add|rename|delete [args]
  export [file]               dump cleartext JSON (careful!)
  import <file>               merge secrets from an export dump
  change-master-password      rotate the master password
  audit [--json]              show the audit log
  status                      vault info
  version

Vault selection:
  --vault <path>              or env GO_PW_VAULT
  --master-password <pw>      or env GO_PW_MASTER_PASSWORD,
  --master-password-file <f>  or interactive prompt
`

type args struct {
	cmd   string
	pos   []string
	flags map[string]string
}

func parseArgs(argv []string) args {
	a := args{flags: map[string]string{}}
	for i := 0; i < len(argv); i++ {
		s := argv[i]
		if strings.HasPrefix(s, "--") {
			key := strings.TrimPrefix(s, "--")
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "--") {
				a.flags[key] = argv[i+1]
				i++
			} else {
				a.flags[key] = "true"
			}
		} else if a.cmd == "" {
			a.cmd = s
		} else {
			a.pos = append(a.pos, s)
		}
	}
	return a
}

func fail(format string, v ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", v...)
	os.Exit(1)
}

func vaultPath(a args) string {
	if p := a.flags["vault"]; p != "" {
		return p
	}
	if p := os.Getenv("GO_PW_VAULT"); p != "" {
		return p
	}
	return "vault.gpw"
}

func masterPassword(a args, confirm bool) string {
	if p := a.flags["master-password"]; p != "" {
		return p
	}
	if f := a.flags["master-password-file"]; f != "" {
		raw, err := os.ReadFile(f)
		if err != nil {
			fail("reading master password file: %v", err)
		}
		return strings.TrimSpace(string(raw))
	}
	if p := os.Getenv("GO_PW_MASTER_PASSWORD"); p != "" {
		return p
	}
	p := promptPassword("Master password: ")
	if confirm {
		if promptPassword("Confirm master password: ") != p {
			fail("passwords do not match")
		}
	}
	return p
}

// promptPassword reads a password without echo when stdin is a terminal;
// prompts go to stderr so stdout stays clean for piping.
func promptPassword(prompt string) string {
	fmt.Fprint(os.Stderr, prompt)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		raw, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			fail("reading password: %v", err)
		}
		return string(raw)
	}
	var line string
	fmt.Fscanln(os.Stdin, &line)
	return strings.TrimSpace(line)
}

func openVault(a args) *vault.Vault {
	path := vaultPath(a)
	v, err := vault.Open(path, masterPassword(a, false))
	if err != nil {
		if errors.Is(err, vault.ErrInvalidPassword) {
			fail("invalid master password")
		}
		fail("opening vault %s: %v", path, err)
	}
	return v
}

// findSecret resolves a positional argument as an ID first, then as a title.
func findSecret(v *vault.Vault, ref string) (vault.Secret, error) {
	if s, err := v.GetSecret(ref, actor); err == nil {
		return s, nil
	}
	return v.GetSecretByTitle(ref, actor)
}

func printJSON(v any) {
	out, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(out))
}

func main() {
	a := parseArgs(os.Args[1:])
	switch a.cmd {
	case "", "help", "--help", "-h":
		fmt.Print(usage)
	case "version":
		fmt.Println("go-pw-cli " + appVersion)
	case "init":
		cmdInit(a)
	case "list":
		cmdList(a)
	case "get":
		cmdGet(a)
	case "set":
		cmdSet(a)
	case "delete":
		cmdDelete(a)
	case "generate":
		cmdGenerate(a)
	case "categories":
		cmdCategories(a)
	case "export":
		cmdExport(a)
	case "import":
		cmdImport(a)
	case "change-master-password":
		cmdChangeMasterPassword(a)
	case "audit":
		cmdAudit(a)
	case "status":
		cmdStatus(a)
	default:
		fail("unknown command %q — run go-pw-cli help", a.cmd)
	}
}

func cmdInit(a args) {
	path := vaultPath(a)
	if len(a.pos) > 0 {
		path = a.pos[0]
	}
	v, err := vault.Create(path, masterPassword(a, true))
	if err != nil {
		fail("%v", err)
	}
	v.Lock()
	fmt.Printf("Vault created at %s\n", path)
}

func cmdList(a args) {
	v := openVault(a)
	defer saveAndLock(v)
	secrets := v.ListSecrets(a.flags["q"])
	if a.flags["json"] != "" {
		printJSON(secrets)
		return
	}
	if len(secrets) == 0 {
		fmt.Println("No secrets.")
		return
	}
	cats := map[string]string{}
	for _, c := range v.ListCategories() {
		cats[c.ID] = c.Name
	}
	for _, s := range secrets {
		line := s.Title
		if s.Username != "" {
			line += "  (" + s.Username + ")"
		}
		if name := cats[s.CategoryID]; name != "" {
			line += "  [" + name + "]"
		}
		fmt.Println(line)
	}
}

func cmdGet(a args) {
	if len(a.pos) == 0 {
		fail("usage: go-pw-cli get <title|id> [--field f] [--json]")
	}
	v := openVault(a)
	defer saveAndLock(v)
	s, err := findSecret(v, a.pos[0])
	if err != nil {
		fail("%v", err)
	}
	if f := a.flags["field"]; f != "" {
		val, err := fieldValue(s, f)
		if err != nil {
			fail("%v", err)
		}
		if a.flags["json"] != "" {
			printJSON(map[string]string{"value": val})
		} else {
			fmt.Println(val)
		}
		return
	}
	if a.flags["json"] != "" {
		printJSON(s)
		return
	}
	fmt.Printf("Title:    %s\n", s.Title)
	fmt.Printf("Username: %s\n", s.Username)
	fmt.Printf("Password: %s\n", s.Password)
	fmt.Printf("URL:      %s\n", s.URL)
	fmt.Printf("Notes:    %s\n", s.Notes)
	fmt.Printf("ID:       %s\n", s.ID)
}

func fieldValue(s vault.Secret, field string) (string, error) {
	switch strings.ToLower(field) {
	case "title":
		return s.Title, nil
	case "username":
		return s.Username, nil
	case "password":
		return s.Password, nil
	case "url":
		return s.URL, nil
	case "notes":
		return s.Notes, nil
	case "id":
		return s.ID, nil
	}
	return "", fmt.Errorf("unknown field %q (title|username|password|url|notes|id)", field)
}

func cmdSet(a args) {
	if len(a.pos) == 0 {
		fail("usage: go-pw-cli set <title> [--username u] [--password p] ...")
	}
	v := openVault(a)
	defer saveAndLock(v)
	title := a.pos[0]
	in := vault.SecretInput{
		Title:      title,
		Username:   a.flags["username"],
		Password:   a.flags["password"],
		URL:        a.flags["url"],
		Notes:      a.flags["notes"],
		CategoryID: a.flags["category"],
	}
	if existing, err := v.GetSecretByTitle(title, actor); err == nil {
		if _, err := v.UpdateSecret(existing.ID, in, actor); err != nil {
			fail("%v", err)
		}
		fmt.Printf("Secret %q updated.\n", title)
	} else {
		if _, err := v.AddSecret(in, actor); err != nil {
			fail("%v", err)
		}
		fmt.Printf("Secret %q saved.\n", title)
	}
}

func cmdDelete(a args) {
	if len(a.pos) == 0 {
		fail("usage: go-pw-cli delete <title|id>")
	}
	v := openVault(a)
	defer saveAndLock(v)
	s, err := findSecret(v, a.pos[0])
	if err != nil {
		fail("%v", err)
	}
	if err := v.DeleteSecret(s.ID, actor); err != nil {
		fail("%v", err)
	}
	fmt.Printf("Secret %q deleted.\n", s.Title)
}

func cmdGenerate(a args) {
	o := vault.DefaultGenerateOptions()
	if l := a.flags["length"]; l != "" {
		n, err := strconv.Atoi(l)
		if err != nil {
			fail("invalid --length %q", l)
		}
		o.Length = n
	}
	if a.flags["no-symbols"] != "" {
		o.Symbols = false
	}
	p, err := vault.GeneratePassword(o)
	if err != nil {
		fail("%v", err)
	}
	fmt.Println(p)
}

func cmdCategories(a args) {
	if len(a.pos) == 0 {
		fail("usage: go-pw-cli categories list|add <name>|rename <id> <name>|delete <id>")
	}
	v := openVault(a)
	defer saveAndLock(v)
	sub, rest := a.pos[0], a.pos[1:]
	switch sub {
	case "list":
		cats := v.ListCategories()
		if a.flags["json"] != "" {
			printJSON(cats)
			return
		}
		if len(cats) == 0 {
			fmt.Println("No categories.")
			return
		}
		for _, c := range cats {
			fmt.Printf("%s  %s\n", c.ID, c.Name)
		}
	case "add":
		if len(rest) < 1 {
			fail("usage: go-pw-cli categories add <name>")
		}
		c, err := v.AddCategory(rest[0], "", actor)
		if err != nil {
			fail("%v", err)
		}
		fmt.Printf("Category %q created (%s).\n", c.Name, c.ID)
	case "rename":
		if len(rest) < 2 {
			fail("usage: go-pw-cli categories rename <id> <new-name>")
		}
		if err := v.RenameCategory(rest[0], rest[1], actor); err != nil {
			fail("%v", err)
		}
		fmt.Println("Category renamed.")
	case "delete":
		if len(rest) < 1 {
			fail("usage: go-pw-cli categories delete <id>")
		}
		if err := v.DeleteCategory(rest[0], actor); err != nil {
			fail("%v", err)
		}
		fmt.Println("Category deleted.")
	default:
		fail("unknown subcommand %q", sub)
	}
}

func cmdExport(a args) {
	v := openVault(a)
	defer saveAndLock(v)
	dump, err := v.Export(actor)
	if err != nil {
		fail("%v", err)
	}
	if len(a.pos) > 0 {
		if err := os.WriteFile(a.pos[0], dump, 0o600); err != nil {
			fail("%v", err)
		}
		fmt.Fprintf(os.Stderr, "Warning: %s contains ALL your secrets in cleartext.\n", a.pos[0])
		return
	}
	fmt.Println(string(dump))
}

func cmdImport(a args) {
	if len(a.pos) == 0 {
		fail("usage: go-pw-cli import <file>")
	}
	raw, err := os.ReadFile(a.pos[0])
	if err != nil {
		fail("%v", err)
	}
	v := openVault(a)
	defer saveAndLock(v)
	n, err := v.Import(raw, actor)
	if err != nil {
		fail("%v", err)
	}
	fmt.Printf("Imported %d secrets.\n", n)
}

func cmdChangeMasterPassword(a args) {
	v := openVault(a)
	defer v.Lock()
	current := masterPassword(a, false)
	newPw := promptPassword("New master password: ")
	if promptPassword("Confirm new master password: ") != newPw {
		fail("passwords do not match")
	}
	if err := v.ChangeMasterPassword(current, newPw); err != nil {
		fail("%v", err)
	}
	fmt.Println("Master password changed.")
}

func cmdAudit(a args) {
	v := openVault(a)
	defer saveAndLock(v)
	log := v.AuditLog()
	if a.flags["json"] != "" {
		printJSON(log)
		return
	}
	for _, e := range log {
		line := fmt.Sprintf("%s  %-4s %s", e.TS, e.Actor, e.Action)
		if e.SecretID != "" {
			line += "  " + e.SecretID
		}
		fmt.Println(line)
	}
}

func cmdStatus(a args) {
	path := vaultPath(a)
	info, err := os.Stat(path)
	if err != nil {
		fail("vault not found at %s", path)
	}
	v := openVault(a)
	defer saveAndLock(v)
	fmt.Printf("Vault:      %s\n", path)
	fmt.Printf("Size:       %d bytes\n", info.Size())
	fmt.Printf("Format:     %s v%d\n", vault.FormatName, vault.FormatVersion)
	fmt.Printf("Secrets:    %d\n", len(v.ListSecrets("")))
	fmt.Printf("Categories: %d\n", len(v.ListCategories()))
	fmt.Printf("Audit:      %d entries\n", len(v.AuditLog()))
}

// saveAndLock persists audit entries added during the command, then wipes the
// in-memory keys.
func saveAndLock(v *vault.Vault) {
	if err := v.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save audit trail: %v\n", err)
	}
	v.Lock()
}
