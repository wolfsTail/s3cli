package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/wolfsTail/s3cli/internal/config"
	"github.com/wolfsTail/s3cli/internal/human"
	"github.com/wolfsTail/s3cli/internal/s3client"
	"github.com/wolfsTail/s3cli/internal/transfer"
)

// точка входа
func Run(argv []string) (int, error) {
	// Глобальные флаги
	var cfgPath string
	var verbose bool
	var noProgress bool

	if len(argv) == 0 {
		printUsage()
		return 0, nil
	}
	rest, err := parseGlobalFlags(argv, &cfgPath, &verbose, &noProgress)
	if err != nil {
		return 4, err
	}
	if len(rest) == 0 {
		printUsage()
		return 0, nil
	}
	switch rest[0] {
	case "-h", "--help", "help":
		printUsage()
		return 0, nil
	case "alias":
		return runAlias(rest[1:], cfgPath)
	case "get":
		return runGet(rest[1:], cfgPath, verbose, !noProgress)
	case "ls":
		return runLs(rest[1:], cfgPath, verbose)
	case "rm":
		return runRm(rest[1:], cfgPath, verbose)
	case "put":
		return runPut(rest[1:], cfgPath, verbose, !noProgress)
	case "stat":
		return runStat(rest[1:], cfgPath, verbose)
	case "cat":
		return runCat(rest[1:], cfgPath, verbose)
	default:
		return 4, fmt.Errorf("неизвестная команда: %q\n\n%s", rest[0], usageShort())
	}
}

func runAlias(args []string, cfgPath string) (int, error) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, aliasUsage())
		return 4, nil
	}
	switch args[0] {
	case "add":
		return aliasAdd(args[1:], cfgPath)
	case "ls":
		return aliasLs(args[1:], cfgPath)
	case "rm":
		return aliasRm(args[1:], cfgPath)
	case "-h", "--help", "help":
		fmt.Print(aliasUsage())
		return 0, nil
	default:
		return 4, fmt.Errorf("неизвестная подкоманда alias: %q\n\n%s", args[0], aliasUsage())
	}
}

func aliasAdd(args []string, cfgPath string) (int, error) {
	// вариант
	// alias add <name> <endpoint> <access_key> <secret_key> [--region EU] [--ssl] [--path-style]
	if len(args) < 4 {
		return 4, fmt.Errorf("недостаточно аргументов для 'alias add'\n\n%s", aliasAddUsage())
	}
	name := args[0]
	endpoint := args[1]
	ak := args[2]
	sk := args[3]

	region := ""
	secure := false
	pathStyle := false

	// остальные...
	var i = 4
	for i < len(args) {
		switch args[i] {
		case "--region":
			if i+1 >= len(args) {
				return 4, fmt.Errorf("флаг --region требует значение\n\n%s", aliasAddUsage())
			}
			region = args[i+1]
			i += 2
		case "--ssl":
			secure = true
			i++
		case "--path-style":
			pathStyle = true
			i++
		case "-h", "--help":
			fmt.Print(aliasAddUsage())
			return 0, nil
		default:
			return 4, fmt.Errorf("неизвестный флаг/аргумент для 'alias add': %q\n\n%s", args[i], aliasAddUsage())
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return 1, err
	}

	err = cfg.AddAlias(name, config.Alias{
		Endpoint:  endpoint,
		Region:    region,
		AccessKey: ak,
		SecretKey: sk,
		Secure:    secure,
		PathStyle: pathStyle,
	})
	if err != nil {
		if errors.Is(err, config.ErrInvalidAlias) {
			return 4, fmt.Errorf("ошибка: некорректное имя алиаса")
		}
		if errors.Is(err, config.ErrAliasExists) {
			return 4, fmt.Errorf("ошибка: алиас %q уже существует. Посмотрите 's3cli alias ls' или удалите его командой 's3cli alias rm %s'", name, name)
		}
		return 1, err
	}

	if err := config.Save(cfgPath, cfg); err != nil {
		return 1, err
	}

	fmt.Printf("Алиас %q добавлен.\n", name)
	return 0, nil
}

func aliasLs(args []string, cfgPath string) (int, error) {
	// Без аргументов
	for _, a := range args {
		switch a {
		case "-h", "--help":
			fmt.Print(aliasLsUsage())
			return 0, nil
		default:
			return 4, fmt.Errorf("лишний аргумент для 'alias ls': %q\n\n%s", a, aliasLsUsage())
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return 1, err
	}
	if len(cfg.Aliases) == 0 {
		fmt.Println("Нет алиасов. Добавьте командой: s3cli alias add <name> <endpoint> <access_key> <secret_key> [--region EU] [--ssl] [--path-style]")
		return 0, nil
	}

	names := make([]string, 0, len(cfg.Aliases))
	for n := range cfg.Aliases {
		names = append(names, n)
	}
	sort.Strings(names)

	fmt.Println("Список алиасов:")
	fmt.Println("NAME\tENDPOINT\tREGION\tSSL\tPATH_STYLE")
	for _, n := range names {
		a := cfg.Aliases[n]
		ssl := "false"
		if a.Secure {
			ssl = "true"
		}
		ps := "false"
		if a.PathStyle {
			ps = "true"
		}
		fmt.Printf("%s\t%s\t\t%s\t%s\t%s\n", n, a.Endpoint, a.Region, ssl, ps)
	}
	return 0, nil
}

func aliasRm(args []string, cfgPath string) (int, error) {
	// Формат: alias rm <name>
	if len(args) == 0 {
		return 4, fmt.Errorf("нужно указать имя алиаса\n\n%s", aliasRmUsage())
	}
	if len(args) > 1 {
		switch args[1] {
		case "-h", "--help":
			fmt.Print(aliasRmUsage())
			return 0, nil
		default:
			return 4, fmt.Errorf("лишний аргумент для 'alias rm': %q\n\n%s", args[1], aliasRmUsage())
		}
	}
	name := args[0]

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return 1, err
	}
	if err := cfg.RemoveAlias(name); err != nil {
		if errors.Is(err, config.ErrAliasNotFound) {
			return 2, fmt.Errorf("ошибка: алиас %q не найден. Посмотрите 's3cli alias ls'", name)
		}
		return 1, err
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		return 1, err
	}
	fmt.Printf("Алиас %q удалён.\n", name)
	return 0, nil
}

func runLs(args []string, cfgPath string, verbose bool) (int, error) {
	// Формат: ls <alias>/<bucket>/<prefix?>
	if len(args) == 0 {
		return 4, fmt.Errorf("нужно указать путь вида alias/bucket[/prefix]\n\nПример:\n  s3cli ls s3s7/my-bucket/reports/2025/")
	}
	if len(args) > 1 {
		switch args[1] {
		case "-h", "--help":
			fmt.Print(lsUsage())
			return 0, nil
		default:
			return 4, fmt.Errorf("лишний аргумент для 'ls': %q\n\n%s", args[1], lsUsage())
		}
	}

	sp, err := parseS3Path(args[0])
	if err != nil {
		return 4, err
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return 1, err
	}
	alias, err := cfg.GetAlias(sp.Alias)
	if err != nil {
		if errors.Is(err, config.ErrAliasNotFound) {
			return 2, fmt.Errorf("ошибка: алиас %q не найден. Посмотрите 's3cli alias ls'", sp.Alias)
		}
		return 1, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := s3client.New(ctx, alias)
	if err != nil {
		return 1, err
	}

	prefix := sp.Key
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
	}

	folders, objects, err := client.ListOneLevel(ctx, sp.Bucket, prefix)
	if err != nil {
		return handleAWSError(err, verbose, "Объекты не найдены", "Доступ запрещён")
	}

	type row struct {
		isDir bool
		name  string
		date  string
		size  string
	}

	rows := make([]row, 0, len(folders)+len(objects))

	for _, f := range folders {
		name := f
		if prefix != "" && strings.HasPrefix(name, prefix) {
			name = strings.TrimPrefix(name, prefix)
		}
		if !strings.HasSuffix(name, "/") {
			name += "/"
		}
		rows = append(rows, row{
			isDir: true,
			name:  name,
			date:  "-",
			size:  "-",
		})
	}

	for _, o := range objects {
		name := o.Key
		if prefix != "" && strings.HasPrefix(name, prefix) {
			name = strings.TrimPrefix(name, prefix)
		}
		rows = append(rows, row{
			isDir: false,
			name:  name,
			date:  human.Time(o.LastModified),
			size:  human.Bytes(o.Size),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].isDir != rows[j].isDir {
			return rows[i].isDir && !rows[j].isDir
		}
		return rows[i].name < rows[j].name
	})

	fmt.Println("date               size       name")

	if len(rows) == 0 {
		fmt.Println("Увы, ничего нет")
		return 0, nil
	}

	for _, r := range rows {
		fmt.Printf("%-16s  %10s  %s\n", r.date, r.size, r.name)
	}

	return 0, nil
}

func runStat(args []string, cfgPath string, verbose bool) (int, error) {
	if len(args) == 0 {
		return 4, fmt.Errorf("нужно указать путь alias/bucket/key\n\n%s", statUsage())
	}
	sp, err := parseS3Path(args[0])
	if err != nil {
		return 4, err
	}
	if sp.Key == "" {
		return 4, fmt.Errorf("нужно указать ключ объекта (а не только бакет/префикс)")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return 1, err
	}
	alias, err := cfg.GetAlias(sp.Alias)
	if err != nil {
		if errors.Is(err, config.ErrAliasNotFound) {
			return 2, fmt.Errorf("ошибка: алиас %q не найден", sp.Alias)
		}
		return 1, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := s3client.New(ctx, alias)
	if err != nil {
		return 1, err
	}

	info, err := client.StatObject(ctx, sp.Bucket, sp.Key)
	if err != nil {
		return handleAWSError(err, verbose,
			fmt.Sprintf("Объект не найден: %s/%s", sp.Bucket, sp.Key),
			"Доступ запрещён",
		)
	}

	fmt.Printf("Key:          %s\n", info.Key)
	fmt.Printf("Size:        %d байт\n", info.Size)
	fmt.Printf("LastModified: %s\n", info.LastModified)
	fmt.Printf("ETag:          %s\n", info.ETag)
	fmt.Printf("Content-Type:  %s\n", info.ContentType)
	return 0, nil
}

func runCat(args []string, cfgPath string, verbose bool) (int, error) {
	if len(args) == 0 {
		return 4, fmt.Errorf("нужно указать путь alias/bucket/key\n\n%s", catUsage())
	}
	sp, err := parseS3Path(args[0])
	if err != nil {
		return 4, err
	}
	if sp.Key == "" {
		return 4, fmt.Errorf("нужно указать ключ объекта (а не только бакет/префикс)")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return 1, err
	}
	alias, err := cfg.GetAlias(sp.Alias)
	if err != nil {
		if errors.Is(err, config.ErrAliasNotFound) {
			return 2, fmt.Errorf("ошибка: алиас %q не найден", sp.Alias)
		}
		return 1, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := s3client.New(ctx, alias)
	if err != nil {
		return 1, err
	}

	if err := client.CatObject(ctx, sp.Bucket, sp.Key, os.Stdout); err != nil {
		return handleAWSError(err, verbose,
			fmt.Sprintf("Объект не найден: %s/%s", sp.Bucket, sp.Key),
			"Доступ запрещён",
		)
	}
	return 0, nil
}

func runRm(args []string, cfgPath string, verbose bool) (int, error) {
	//   rm <alias>/<bucket>/<key> — удалить объект
	//   rm -r <alias>/<bucket>/<prefix/> — удалить рекурсивно по префиксу
	if len(args) == 0 {
		return 4, fmt.Errorf("нужно указать путь alias/bucket/key или -r alias/bucket/prefix/\n\n%s", rmUsage())
	}

	recursive := false
	pos := 0
	if args[0] == "-r" || args[0] == "--recursive" {
		recursive = true
		pos = 1
	}

	if len(args) <= pos {
		return 4, fmt.Errorf("не указан путь для удаления\n\n%s", rmUsage())
	}
	target := args[pos]

	sp, err := parseS3Path(target)
	if err != nil {
		return 4, err
	}
	if sp.Key == "" {
		return 4, fmt.Errorf("нужно указать ключ или префикс, а не только алиас/бакет")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return 1, err
	}
	alias, err := cfg.GetAlias(sp.Alias)
	if err != nil {
		if errors.Is(err, config.ErrAliasNotFound) {
			return 2, fmt.Errorf("ошибка: алиас %q не найден. Уточни 's3cli alias ls'", sp.Alias)
		}
		return 1, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	client, err := s3client.New(ctx, alias)
	if err != nil {
		return 1, err
	}

	if recursive {
		// ?!
		if sp.Key == "" || sp.Key == "/" {
			return 4, fmt.Errorf("Подумаф ещё, удаление бакета того не стоит...")
		}
		prefix := sp.Key
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		n, err := client.DeletePrefix(ctx, sp.Bucket, prefix)
		if err != nil {
			return handleAWSError(err, verbose, "Не найдено", "Доступ запрещён")
		}
		fmt.Printf("Удалено объектов: %d\n", n)
		return 0, nil
	}
	if strings.HasSuffix(sp.Key, "/") {
		return 4, fmt.Errorf("Хочешь удлить весь префикс>? Используйте флаг -r.")
	}
	if err := client.DeleteObject(ctx, sp.Bucket, sp.Key); err != nil {
		return handleAWSError(err, verbose,
			fmt.Sprintf("Объект не найден: %s/%s", sp.Bucket, sp.Key),
			"Доступ запрещён",
		)
	}
	fmt.Println("Удалено.")
	return 0, nil
}

func runPut(args []string, cfgPath string, verbose bool, showProgress bool) (int, error) {
	// put <local_path> <alias>/<bucket>/<key|prefix/> [-j N]
	if len(args) < 2 {
		return 4, fmt.Errorf("нужно указать локальный путь и целевой путь вида alias/bucket/key|prefix/\n\n%s", putUsage())
	}

	localPath := args[0]
	dest := args[1]
	jobs := 4

	// парсинг
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "-j", "--jobs":
			if i+1 >= len(args) {
				return 4, fmt.Errorf("флаг -j требует число")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n <= 0 {
				return 4, fmt.Errorf("некорректное значение для -j: %q", args[i+1])
			}
			jobs = n
			i++
		case "-h", "--help":
			fmt.Print(putUsage())
			return 0, nil
		default:
			return 4, fmt.Errorf("неизвестный аргумент для put: %q\n\n%s", args[i], putUsage())
		}
	}

	sp, err := parseS3Path(dest)
	if err != nil {
		return 4, err
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return 1, err
	}
	alias, err := cfg.GetAlias(sp.Alias)
	if err != nil {
		if errors.Is(err, config.ErrAliasNotFound) {
			return 2, fmt.Errorf("ошибка: алиас %q не найден", sp.Alias)
		}
		return 1, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()

	client, err := s3client.New(ctx, alias)
	if err != nil {
		return 1, err
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return 1, fmt.Errorf("не удалось получить информацию о %q: %w", localPath, err)
	}

	if info.IsDir() {
		prefix := sp.Key
		if prefix == "" || !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		stats, err := transfer.UploadTree(ctx, client.S3, sp.Bucket, prefix, localPath, jobs, showProgress)
		if err != nil {
			return handleAWSError(err, verbose, "Не найдено", "Доступ запрещён")
		}
		fmt.Printf("Файлов: %d, загружено: %d, ошибок: %d\n", stats.TotalFiles, stats.Uploaded, stats.Failed)
		if stats.Failed > 0 {
			return 1, fmt.Errorf("почти... часть файлов не загружена")
		}
		return 0, nil
	}

	key := sp.Key
	if key == "" || strings.HasSuffix(key, "/") {
		base := filepath.Base(localPath)
		key = strings.TrimSuffix(key, "/")
		if key != "" {
			key += "/" + base
		} else {
			key = base
		}
	}
	if err := transfer.UploadFile(ctx, client.S3, sp.Bucket, key, localPath, showProgress); err != nil {
		return handleAWSError(err, verbose, "Не найдено", "Доступ запрещён")
	}
	fmt.Println("Загружено.")
	return 0, nil
}

func runGet(args []string, cfgPath string, verbose bool, showProgress bool) (int, error) {
	// get <alias>/<bucket>/<key|prefix/> <local_path> [-j N]
	if len(args) < 2 {
		return 4, fmt.Errorf("нужно указать источник alias/bucket/key|prefix/ и локальный путь\n\n%s", getUsage())
	}

	source := args[0]
	localRoot := args[1]
	jobs := 4

	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "-j", "--jobs":
			if i+1 >= len(args) {
				return 4, fmt.Errorf("флаг -j требует число")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n <= 0 {
				return 4, fmt.Errorf("некорректное значение для -j: %q", args[i+1])
			}
			jobs = n
			i++
		case "-h", "--help":
			fmt.Print(getUsage())
			return 0, nil
		default:
			return 4, fmt.Errorf("неизвестный аргумент для get: %q\n\n%s", args[i], getUsage())
		}
	}

	sp, err := parseS3Path(source)
	if err != nil {
		return 4, err
	}
	if sp.Key == "" {
		return 4, fmt.Errorf("нужно указать ключ или префикс для скачивания")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return 1, err
	}
	alias, err := cfg.GetAlias(sp.Alias)
	if err != nil {
		if errors.Is(err, config.ErrAliasNotFound) {
			return 2, fmt.Errorf("ошибка: алиас %q не найден", sp.Alias)
		}
		return 1, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()

	client, err := s3client.New(ctx, alias)
	if err != nil {
		return 1, err
	}

	if strings.HasSuffix(sp.Key, "/") {
		keys, err := client.ListAllKeys(ctx, sp.Bucket, sp.Key)
		if err != nil {
			return handleAWSError(err, verbose, "не найдено", "Доступ запрещён")
		}
		if len(keys) == 0 {
			fmt.Println("Под данным префиксом объектов не найдено!")
			return 2, nil
		}
		stats, err := transfer.DownloadKeys(ctx, client.S3, sp.Bucket, keys, sp.Key, localRoot, jobs, showProgress)
		if err != nil {
			return 1, err
		}
		fmt.Printf("Файлов: %d, скачано: %d, ошибок: %d\n", stats.TotalFiles, stats.Downloaded, stats.Failed)
		if stats.Failed > 0 {
			return 1, fmt.Errorf("ну почти... часть файлов не скачана")
		}
		return 0, nil
	}

	dest := localRoot
	fi, err := os.Stat(localRoot)
	if err == nil && fi.IsDir() {
		base := filepath.Base(sp.Key)
		dest = filepath.Join(localRoot, base)
	}
	if err := transfer.DownloadFile(ctx, client.S3, sp.Bucket, sp.Key, dest, showProgress); err != nil {
		return handleAWSError(err, verbose, "не найдено", "Доступ запрещён")
	}
	fmt.Println("Скачано.")
	return 0, nil
}

func parseGlobalFlags(argv []string, cfgPath *string, verbose *bool, noProgress *bool) ([]string, error) {
	out := make([]string, 0, len(argv))
	for i := 0; i < len(argv); i++ {
		switch argv[i] {
		case "--config":
			if i+1 >= len(argv) {
				return nil, fmt.Errorf("флаг --config требует путь к файлу")
			}
			*cfgPath = argv[i+1]
			i++
		case "-v", "--verbose":
			*verbose = true
		case "--no-progress":
			*noProgress = true
		case "-h", "--help":
			out = append(out, argv[i])
		default:
			out = append(out, argv[i])
		}
	}
	return out, nil
}

// справка

func usageShort() string {
	var b strings.Builder
	b.WriteString("Использование:\n")
	b.WriteString("  s3cli [глобальные флаги] <команда> [аргументы]\n\n")
	b.WriteString("Команды:\n")
	b.WriteString("  alias add <name> <endpoint> <access_key> <secret_key> [--region EU] [--ssl] [--path-style]\n")
	b.WriteString("  alias ls\n")
	b.WriteString("  alias rm <name>\n\n")
	b.WriteString("  ls <alias>/<bucket>/<prefix?>\n\n")
	b.WriteString("Глобальные флаги:\n")
	b.WriteString("  --config PATH      Конфиг -> (по умолчанию ~/.s3cli/config.yaml)\n")
	b.WriteString("  --no-progress      Отключить прогресс-бар\n")
	b.WriteString("  -v, --verbose      Подробные ошибки/детали\n")
	b.WriteString("  -h, --help         Справка\n")
	return b.String()
}

func handleAWSError(err error, verbose bool, notFoundMsg, forbiddenMsg string) (int, error) {
	var re *smithyhttp.ResponseError
	if errors.As(err, &re) {
		status := re.HTTPStatusCode()
		switch status {
		case 403:
			if verbose {
				return 3, fmt.Errorf("%s (HTTP 403): %v", orDefault(forbiddenMsg, "Доступ запрещён"), err)
			}
			return 3, fmt.Errorf("%s (HTTP 403)", orDefault(forbiddenMsg, "Доступ запрещён"))
		case 404:
			if verbose {
				return 2, fmt.Errorf("%s (HTTP 404): %v", orDefault(notFoundMsg, "Не найдено"), err)
			}
			return 2, fmt.Errorf("%s (HTTP 404)", orDefault(notFoundMsg, "Не найдено"))
		case 400, 405, 409:
			if verbose {
				return 4, fmt.Errorf("неверные аргументы (HTTP %d): %v", status, err)
			}
			return 4, fmt.Errorf("неверные аргументы (HTTP %d)", status)
		default:
			if verbose {
				return 1, fmt.Errorf("ошибка S3 (HTTP %d): %v", status, err)
			}
			return 1, fmt.Errorf("ошибка S3 (HTTP %d)", status)
		}
	}
	var ae smithy.APIError
	if errors.As(err, &ae) && verbose {
		return 1, fmt.Errorf("ошибка S3: %s — %s: %v", ae.ErrorCode(), ae.ErrorMessage(), err)
	}
	if verbose {
		return 1, fmt.Errorf("ошибка: %v", err)
	}
	return 1, fmt.Errorf("ошибка, хз ваще какая")
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func printUsage() {
	fmt.Print(usageShort())
}

func aliasUsage() string {
	return `Использование:
  s3cli alias <подкоманда> [аргументы]

Подкоманды:
  add <name> <endpoint> <access_key> <secret_key> [--region EU] [--ssl] [--path-style]
  ls
  rm <name>

Примеры:
  s3cli alias add s3s7 https://s3.s7corp.ru ACCESS_KEY SECRET_KEY --region eu-central-1 --ssl --path-style
  s3cli alias ls
  s3cli alias rm s3s7
`
}

func aliasAddUsage() string {
	return `Использование:
  s3cli alias add <name> <endpoint> <access_key> <secret_key> [--region EU] [--ssl] [--path-style]

Описание:
  Добавляет алиас подключения к S3-совместимому хранилищу.
`
}

func aliasLsUsage() string {
	return `Использование:
  s3cli alias ls

Описание:
  Показывает список настроенных алиасов.
`
}

func aliasRmUsage() string {
	return `Использование:
  s3cli alias rm <name>

Описание:
  Удаляет указанный алиас.
`
}

func lsUsage() string {
	return `Использование:
  s3cli ls <alias>/<bucket>/<prefix?>

Описание:
  Показывает префиксы и объекты одним уровнем глубины.
Пример:
  s3cli ls s3s7/fao_qa/reports/2025/
`
}
func statUsage() string {
	return `Использование:
  s3cli stat <alias>/<bucket>/<key>

Описание:
  Показывает метаданные объекта.
`
}

func catUsage() string {
	return `Использование:
  s3cli cat <alias>/<bucket>/<key>

Описание:
  Выводит содержимое объекта в stdout.
`
}

func rmUsage() string {
	return `Использование:
  s3cli rm <alias>/<bucket>/<key>
  s3cli rm -r <alias>/<bucket>/<prefix/>

Описание:
  Удаляет объект или все объекты под заданным префиксом (-r)
  Чтобы "не натворить дел" пустой префикс не допускается!
`
}

func getUsage() string {
	return `Использование:
  s3cli get <alias>/<bucket>/<key|prefix/> <local_path> [-j N]

Описание:
  Скачивает один объект или весь префикс (рекурсивно) по указанному пути
  -j N — число параллельных загрузок (по умолчанию 4).
`
}

func putUsage() string {
	return `Использование:
  s3cli put <local_path> <alias>/<bucket>/<key|prefix/> [-j N]

Описание:
  Загрузка файла или всего каталога (рекурсивно) в указанный бакет/префикс.
  -j N — число параллельных загрузок (по умолчанию 4).
`
}
