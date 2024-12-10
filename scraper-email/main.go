package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"net/smtp"
	"sync"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"os/exec"
    "runtime"

	"github.com/gosom/scrapemate"
	"github.com/gosom/scrapemate/scrapemateapp"
	"github.com/gosom/google-maps-scraper/gmaps"
	"github.com/fatih/color"
)

type EmailConfig struct {
    Subject string   `json:"subject"`
    Body    []string `json:"body"` // Modifica da string a []string
}

func clearTerminal() {
    var clearCmd *exec.Cmd

    // Usa comandi diversi in base al sistema operativo
    if runtime.GOOS == "windows" {
        clearCmd = exec.Command("cmd", "/c", "cls") // Windows
    } else {
        clearCmd = exec.Command("clear") // Linux e macOS
    }

    clearCmd.Stdout = os.Stdout
    _ = clearCmd.Run() // Esegui il comando e ignora eventuali errori
}

func printUsage() {
    banner := `
================================================================================
                             EFFEMMEWEB UTILITY
================================================================================`

    // Usa colori per evidenziare
    title := color.New(color.FgCyan, color.Bold).SprintFunc()
    section := color.New(color.FgGreen).SprintFunc()
    highlight := color.New(color.FgYellow, color.Bold).SprintFunc()

    fmt.Println(title(banner))
    fmt.Println(title("Benvenuto nel programma per scraping, gestione CSV, generazione SQL e invio email.\n"))
    fmt.Println(section("COME SI USA:"))
    fmt.Println("1. Assicurati che i file di configurazione richiesti siano presenti nella directory:")
    fmt.Printf("   - %s: %s\n", highlight("keyword.csv"), "Contiene le parole chiave per lo scraping.")
    fmt.Printf("   - %s: %s\n", highlight("comuni.csv"), "Contiene l'elenco dei comuni per combinazioni di ricerca.")
    fmt.Printf("   - %s: %s\n", highlight("email_config.json"), "File JSON con configurazione email (subject e body).\n")

    fmt.Println(section("2. Funzionalità disponibili:"))
    fmt.Printf("   a. %s\n", highlight("Scraping:"))
    fmt.Println("      - Genera un file CSV con i risultati dello scraping da Google Maps.")
    fmt.Println("      - Include dettagli come Nome Attività, Categoria, Sito Web, Telefono, ecc.")
    fmt.Printf("   b. %s\n", highlight("Generazione SQL:"))
    fmt.Println("      - Converte i risultati del CSV in istruzioni SQL per il database.")
    fmt.Printf("   c. %s\n", highlight("Invio Email:"))
    fmt.Println("      - Legge un CSV di destinatari e invia email personalizzate tramite un server SMTP.\n")

    fmt.Println(section("3. ESECUZIONE:"))
    fmt.Println("   - Lancia il programma con:")
    fmt.Printf("     %s\n", highlight("go run main.go"))
    fmt.Println("   - Segui le istruzioni interattive sul terminale per scegliere cosa fare.\n")

    fmt.Println(section("NOTA:"))
    fmt.Println("- Puoi interrompere l'esecuzione in sicurezza con CTRL+C.")
    fmt.Println("- Assicurati di avere una connessione a internet per lo scraping e l'invio email.")
    fmt.Println(title(banner))
}

func loadEmailConfig(filePath string) (EmailConfig, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return EmailConfig{}, fmt.Errorf("impossibile leggere il file di configurazione email: %v", err)
	}

	var config EmailConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return EmailConfig{}, fmt.Errorf("errore durante il parsing del file JSON: %v", err)
	}

	return config, nil
}

func main() {
    clearTerminal()
    // Stampa il banner di benvenuto e le istruzioni
    printUsage()

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Variabile per sapere se un file CSV è stato generato
    var generatedCSV string

    // Gestione dei segnali
    signalChan := make(chan os.Signal, 1)
    signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

    go func() {
        <-signalChan
        fmt.Println("\nCTRL+C rilevato. Attendi...")
        cancel()

        if generatedCSV != "" {
            fmt.Println(color.New(color.FgYellow).Sprint("Pulizia del file CSV in corso..."))
            if err := cleanLastRowFromCSV(generatedCSV); err != nil {
                fmt.Println(color.New(color.FgRed).Sprintf("Errore durante la pulizia del CSV: %v", err))
            } else {
                fmt.Println(color.New(color.FgGreen).Sprint("Pulizia completata con successo."))
            }
        }

        fmt.Println("Uscita dal programma.")
        os.Exit(0)
    }()

    // Avvio del menu principale
    for {
        fmt.Println("\nCosa vuoi fare?")
        fmt.Println("1. Avvia scraping")
        fmt.Println("2. Genera file SQL")
        fmt.Println("3. Genera file email")
        fmt.Println("4. Invia email")
        fmt.Println("5. Esci")
        fmt.Print("Scegli un'opzione (1-5): ")

        reader := bufio.NewReader(os.Stdin)
        choice, _ := reader.ReadString('\n')
        choice = strings.TrimSpace(choice)

        switch choice {
        case "1":
            if confirmAction("Vuoi procedere con lo scraping?") {
                if err := runScrapingFlow(ctx, &generatedCSV); err != nil {
                    fmt.Println(color.New(color.FgRed).Sprintf("Errore durante lo scraping: %v", err))
                }
            }
        case "2":
            if generatedCSV == "" {
                fmt.Println(color.New(color.FgYellow).Sprint("Nessun file CSV disponibile. Avvia prima lo scraping."))
                continue
            }
            if confirmAction("Vuoi generare il file SQL?") {
                if err := generateSQLFromCSV(ctx, generatedCSV); err != nil {
                    fmt.Println(color.New(color.FgRed).Sprintf("Errore durante la generazione del file SQL: %v", err))
                }
            }
        case "3":
            if generatedCSV == "" {
                fmt.Println(color.New(color.FgYellow).Sprint("Nessun file CSV disponibile. Avvia prima lo scraping."))
                continue
            }
            if confirmAction("Vuoi generare il file email?") {
                if err := generateEmailsToSend(generatedCSV, "Categoria"); err != nil {
                    fmt.Println(color.New(color.FgRed).Sprintf("Errore durante la generazione del file email: %v", err))
                }
            }
        case "4":
            emailCSVPath := fmt.Sprintf("email_results/emails_to_send_Categoria_%s.csv", time.Now().Format("20060102_150405"))
            logPath := "sendmaillog.csv"
            smtpConfig := map[string]string{
                "server":   "mail.effemmeweb.it",
                "port":     "465",
                "user":     "info@effemmeweb.it",
                "password": "Ludovica2021",
            }
            if confirmAction("Vuoi inviare le email?") {
                if err := processEmails(ctx, emailCSVPath, logPath, smtpConfig); err != nil {
                    fmt.Println(color.New(color.FgRed).Sprintf("Errore durante l'invio delle email: %v", err))
                }
            }
        case "5":
            fmt.Println(color.New(color.FgGreen).Sprint("Uscita dal programma. Arrivederci!"))
            return
        default:
            fmt.Println(color.New(color.FgRed).Sprint("Opzione non valida. Riprova."))
        }
    }
}

// Funzione per confermare un'azione
func confirmAction(message string) bool {
    fmt.Printf("%s (y/n): ", message)
    reader := bufio.NewReader(os.Stdin)
    choice, _ := reader.ReadString('\n')
    choice = strings.TrimSpace(strings.ToLower(choice))
    return choice == "y"
}

func runScrapingFlow(ctx context.Context, generatedCSV *string) error {
    fmt.Println("Avvio dello scraping...")
    newCSV, err := runScraping(ctx)
    if err != nil {
        return fmt.Errorf("Errore durante lo scraping: %v", err)
    }

    fmt.Printf("Nuovo file CSV generato: %s\n", newCSV)
    *generatedCSV = newCSV // Memorizza il file generato
    return nil
}

func processCSV(ctx context.Context, csvPath string) error {
	// Gestione delle domande per il file CSV
	if askUser("Vuoi generare il file SQL? (y/n): ") {
		if err := generateSQLFromCSV(ctx, csvPath); err != nil {
			return fmt.Errorf("Errore durante la generazione del file SQL: %v", err)
		}
		fmt.Println("File SQL generato con successo.")
	}

	if askUser("Vuoi generare il file email? (y/n): ") {
		if err := generateEmailsToSend(csvPath, "Categoria"); err != nil {
			return fmt.Errorf("Errore durante la generazione del file email: %v", err)
		}
		fmt.Println("File email generato con successo.")
	}

	if askUser("Vuoi inviare le email dall'account configurato? (y/n): ") {
		emailCSVPath := fmt.Sprintf("email_results/emails_to_send_Categoria_%s.csv", time.Now().Format("20060102_150405"))
		logPath := "sendmaillog.csv"
		smtpConfig := map[string]string{
			"server":   "mail.effemmeweb.it",
			"port":     "465",
			"user":     "info@effemmeweb.it",
			"password": "Ludovica2021",
		}
		if err := processEmails(ctx, emailCSVPath, logPath, smtpConfig); err != nil {
			return fmt.Errorf("Errore durante l'invio delle email: %v", err)
		}
		fmt.Println("Email inviate con successo.")
	}

	return nil
}

func isCSVComplete(filePath string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("impossibile aprire il file CSV: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	_, err = reader.Read() // Controlla se esiste l'intestazione
	if err == io.EOF {
		return false, nil // Il file è vuoto o incompleto
	}
	if err != nil {
		return false, fmt.Errorf("errore nella lettura del file CSV: %v", err)
	}
	return true, nil
}

func handleExit(ctx context.Context, csvPath string) {
	fmt.Println("\nOperazioni su file CSV interrotte. Procedi con i seguenti passaggi:")

	// Pulisci l'ultima riga del CSV
	err := cleanLastRowFromCSV(csvPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Errore durante la pulizia del CSV: %v\n", err)
		return
	}

	// Procedi con le operazioni sul CSV
	if err := processCSV(ctx, csvPath); err != nil {
		fmt.Fprintf(os.Stderr, "Errore durante la gestione del CSV: %v\n", err)
	}
}

func askUser(prompt string) bool {
	for {
		fmt.Print(prompt)
		reader := bufio.NewReader(os.Stdin)
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(strings.ToLower(choice))

		if choice == "y" {
			return true
		} else if choice == "n" {
			return false
		} else {
			fmt.Println("Input non valido. Rispondi con 'y' o 'n'.")
		}
	}
}

func findLastGeneratedCSV() (string, error) {
	// Cerca file CSV con il formato nomecategoria_data_ora.csv all'interno della directory csv_results
	csvDir := "csv_results"
	files, err := filepath.Glob(filepath.Join(csvDir, "*_*.csv"))
	if err != nil {
		return "", fmt.Errorf("errore durante la ricerca dei file CSV nella directory %s: %v", csvDir, err)
	}

	if len(files) > 0 {
		// Trova il file più recente
		var latestFile string
		var latestModTime time.Time

		for _, file := range files {
			info, err := os.Stat(file)
			if err != nil {
				continue
			}
			if info.ModTime().After(latestModTime) {
				latestModTime = info.ModTime()
				latestFile = file
			}
		}

		return latestFile, nil
	}

	return "", nil // Nessun file trovato
}

func runScraping(ctx context.Context) (string, error) {
    // Creazione della directory per i CSV
    csvDir := "csv_results"
    if err := os.MkdirAll(csvDir, os.ModePerm); err != nil {
        return "", fmt.Errorf("errore nella creazione della directory %s: %v", csvDir, err)
    }

    keywordFile := "./keyword.csv"
    comuniFile := "./comuni.csv"

    keywords, err := readCSVColumn(keywordFile, "keyword")
    if err != nil {
        return "", fmt.Errorf("errore durante la lettura del file delle keyword: %v", err)
    }

    if len(keywords) == 0 {
        return "", fmt.Errorf("nessuna keyword trovata")
    }

    categoryName := keywords[0]
    currentTime := time.Now().Format("20060102_150405")
    outputFileName := fmt.Sprintf("%s/%s_%s.csv", csvDir, categoryName, currentTime)

    output, err := os.Create(outputFileName)
    if err != nil {
        return "", fmt.Errorf("errore durante la creazione del file CSV: %v", err)
    }
    defer output.Close()

    csvWriter := csv.NewWriter(output)
    defer csvWriter.Flush()

    csvWriter.Write([]string{
		"Nome Attività",
		"Categoria",
		"Sito Web",
		"Telefono",
		"Indirizzo",
		"Comune",
		"Provincia",
		"Email",
		"Protocollo",   // Aggiunto protocollo
		"Tecnologia",   // Aggiunta tecnologia
		"Cookie Banner", // Aggiunto Cookie Banner
	})

    writers := []scrapemate.ResultWriter{
        NewCustomCsvWriter(csvWriter),
    }

    opts := []func(*scrapemateapp.Config) error{
        scrapemateapp.WithConcurrency(4),
        scrapemateapp.WithExitOnInactivity(3 * time.Minute),
        scrapemateapp.WithJS(scrapemateapp.DisableImages()),
    }

    cfg, err := scrapemateapp.NewConfig(writers, opts...)
    if err != nil {
        return "", fmt.Errorf("errore durante la configurazione dello scraping: %v", err)
    }

    app, err := scrapemateapp.NewScrapeMateApp(cfg)
    if err != nil {
        return "", fmt.Errorf("errore durante l'inizializzazione dello scraping: %v", err)
    }

    keywordJobs, err := createKeywordJobs("en", keywordFile, comuniFile, 10, true)
    if err != nil {
        return "", fmt.Errorf("errore durante la creazione dei lavori di scraping: %v", err)
    }

    if len(keywordJobs) == 0 {
        return "", fmt.Errorf("nessun lavoro di scraping creato")
    }

    jobs := convertToJobs(keywordJobs)

    // Avvia lo scraping con il contesto
    if err := app.Start(ctx, jobs...); err != nil && ctx.Err() != context.Canceled {
        return "", fmt.Errorf("errore durante lo scraping: %v", err)
    }

    return outputFileName, nil
}

func cleanLastRowFromCSV(filePath string) error {
    file, err := os.Open(filePath)
    if err != nil {
        return fmt.Errorf("impossibile aprire il file: %v", err)
    }
    defer file.Close()

    reader := csv.NewReader(file)
    var rows [][]string
    var numHeaders int

    // Leggi l'intestazione
    header, err := reader.Read()
    if err != nil {
        return fmt.Errorf("errore nella lettura dell'intestazione del CSV: %v", err)
    }
    rows = append(rows, header)
    numHeaders = len(header)

    // Leggi le righe del CSV
    for {
        record, err := reader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            fmt.Printf("Errore durante la lettura di una riga: %v\n", err)
            continue
        }

        // Controlla che il numero di campi sia corretto
        if len(record) != numHeaders {
            fmt.Printf("Riga ignorata (numero di campi errato): %v\n", record)
            continue
        }
        rows = append(rows, record)
    }

    // Rimuove l'ultima riga (se necessaria)
    if len(rows) > 1 {
        rows = rows[:len(rows)-1]
    }

    output, err := os.Create(filePath)
    if err != nil {
        return fmt.Errorf("errore durante la creazione del CSV: %v", err)
    }
    defer output.Close()

    writer := csv.NewWriter(output)
    defer writer.Flush()

    for _, row := range rows {
        if err := writer.Write(row); err != nil {
            return fmt.Errorf("errore durante la scrittura del CSV: %v", err)
        }
    }

    return nil
}

func generateSQLFromCSV(ctx context.Context, csvFilePath string) error {
	sqlDir := "sql_results"
	if err := os.MkdirAll(sqlDir, os.ModePerm); err != nil {
		return fmt.Errorf("errore nella creazione della directory %s: %v", sqlDir, err)
	}

	tableName := "EFFEMMEWEB_RUBRICA"
	columns := []string{
		"EFFEMMEWEB_NOME",
		"EFFEMMEWEB_CATEGORIA",
		"EFFEMMEWEB_NOME_DOMINIO",
		"EFFEMMEWEB_TELEFONO1",
		"EFFEMMEWEB_INDIRIZZO",
		"EFFEMMEWEB_COMUNE",
		"EFFEMMEWEB_PROVINCIA",
		"EFFEMMEWEB_EMAIL",
	}

	// Estrai il valore della categoria dal file keyword.csv
	keywordFile := "./keyword.csv"
	fixedCategoryValue, err := extractFirstKeyword(keywordFile)
	if err != nil {
		return fmt.Errorf("errore durante l'estrazione della categoria: %v", err)
	}

	currentTime := time.Now().Format("20060102_150405")
	outputSQLPath := fmt.Sprintf("%s/output_%s_%s.sql", sqlDir, fixedCategoryValue, currentTime)

	err = generateSQL(csvFilePath, outputSQLPath, tableName, columns, fixedCategoryValue)
	if err != nil {
		return fmt.Errorf("errore durante la generazione del file SQL: %v", err)
	}

	fmt.Printf("File SQL generato con successo: %s\n", outputSQLPath)

	return nil
}

func extractFirstKeyword(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("impossibile aprire il file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'

	headers, err := reader.Read()
	if err != nil {
		return "", fmt.Errorf("errore nella lettura dell'intestazione: %v", err)
	}

	columnIndex := -1
	for i, col := range headers {
		if strings.ToLower(strings.TrimSpace(col)) == "keyword" {
			columnIndex = i
			break
		}
	}

	if columnIndex == -1 {
		return "", fmt.Errorf("colonna 'keyword' non trovata")
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("errore nella lettura del file CSV: %v", err)
		}

		if columnIndex < len(record) {
			keyword := strings.TrimSpace(record[columnIndex])
			if keyword != "" {
				return keyword, nil
			}
		}
	}

	return "", fmt.Errorf("nessuna keyword valida trovata")
}

func generateEmailsToSend(csvFilePath string, category string) error {
	emailDir := "email_results"
	if err := os.MkdirAll(emailDir, os.ModePerm); err != nil {
		return fmt.Errorf("errore nella creazione della directory %s: %v", emailDir, err)
	}

	currentTime := time.Now().Format("20060102_150405")
	outputEmailFilePath := fmt.Sprintf("%s/emails_to_send_%s_%s.csv", emailDir, category, currentTime)

	inputFile, err := os.Open(csvFilePath)
	if err != nil {
		return fmt.Errorf("impossibile aprire il file CSV: %v", err)
	}
	defer inputFile.Close()

	reader := csv.NewReader(inputFile)
	headers, err := reader.Read()
	if err != nil {
		return fmt.Errorf("errore nella lettura dell'intestazione CSV: %v", err)
	}

	emailIndex, nameIndex := -1, -1
	for i, header := range headers {
		switch strings.ToLower(strings.TrimSpace(header)) {
		case "email":
			emailIndex = i
		case "nome attività":
			nameIndex = i
		}
	}

	if emailIndex == -1 || nameIndex == -1 {
		return fmt.Errorf("colonne 'email' o 'nome attività' non trovate nel file CSV")
	}

	outputFile, err := os.Create(outputEmailFilePath)
	if err != nil {
		return fmt.Errorf("errore nella creazione del file email CSV: %v", err)
	}
	defer outputFile.Close()

	writer := csv.NewWriter(outputFile)
	defer writer.Flush()

	writer.Write([]string{"Email", "Nome Attività"})

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("errore nella lettura del file CSV: %v", err)
		}

		if len(record) > emailIndex && len(record) > nameIndex {
			email := strings.TrimSpace(record[emailIndex])
			name := strings.TrimSpace(record[nameIndex])

			if email != "" && name != "" {
				if err := writer.Write([]string{email, name}); err != nil {
					return fmt.Errorf("errore durante la scrittura del file email CSV: %v", err)
				}
			}
		}
	}

	fmt.Printf("File delle email generato con successo: %s\n", outputEmailFilePath)
	return nil
}

func sendEmail(to, subject, body, smtpServer, smtpPort, smtpUser, smtpPassword string) error {
	auth := smtp.PlainAuth("", smtpUser, smtpPassword, smtpServer)

	// Connessione TLS per porta 465
	conn, err := tls.Dial("tcp", smtpServer+":"+smtpPort, &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         smtpServer,
	})
	if err != nil {
		return fmt.Errorf("errore durante la connessione al server SMTP: %v", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, smtpServer)
	if err != nil {
		return fmt.Errorf("errore durante la creazione del client SMTP: %v", err)
	}
	defer client.Close()

	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("errore durante l'autenticazione SMTP: %v", err)
	}

	msg := fmt.Sprintf("From: %s\nTo: %s\nSubject: %s\nMIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n%s", smtpUser, to, subject, body)

	if err = client.Mail(smtpUser); err != nil {
		return fmt.Errorf("errore durante la definizione del mittente: %v", err)
	}

	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("errore durante la definizione del destinatario: %v", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("errore durante l'invio dei dati: %v", err)
	}

	_, err = w.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("errore durante la scrittura del messaggio: %v", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("errore durante la chiusura del writer: %v", err)
	}

	return client.Quit()
}

func readSendMailLog(filePath string) (map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil // File non esiste, ritorna una mappa vuota
		}
		return nil, fmt.Errorf("errore durante l'apertura del file di log: %v", err)
	}
	defer file.Close()

	log := make(map[string]string)
	reader := csv.NewReader(file)

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if len(record) < 2 {
			continue
		}
		log[record[0]] = record[1] // Email -> Stato
	}

	return log, nil
}

func updateSendMailLog(filePath, email, status string) error {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("errore durante l'apertura del file di log: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{email, status}); err != nil {
		return fmt.Errorf("errore durante la scrittura nel file di log: %v", err)
	}

	return nil
}

func sendEmailsFromQueue(ctx context.Context, emailQueue <-chan []string, smtpConfig map[string]string, logPath string, emailConfigPath string, wg *sync.WaitGroup) {
    defer wg.Done()

    ticker := time.NewTicker(time.Second * 3600 / 100) // Rate limit: 100 email/ora
    defer ticker.Stop()

    // Carica il subject e il body dal file JSON
    emailConfig, err := loadEmailConfig(emailConfigPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Errore durante il caricamento della configurazione email: %v\n", err)
        return
    }

    for {
        select {
        case <-ctx.Done():
            return
        case emailData, ok := <-emailQueue:
            if !ok {
                return
            }

            email := emailData[0]
            name := emailData[1]
            fmt.Printf("Elaborazione email: %s - Nome: %s\n", email, name)

            // Leggi il log e verifica se l'email è già stata inviata
            log, err := readSendMailLog(logPath)
            if err != nil {
                fmt.Fprintf(os.Stderr, "Errore nella lettura del log: %v\n", err)
                continue
            }
            if _, exists := log[email]; exists {
                fmt.Printf("Email già inviata: %s\n", email)
                continue
            }

            // Personalizza il subject e il body
            subject := strings.ReplaceAll(emailConfig.Subject, "{name}", name)
            body := strings.Join(emailConfig.Body, "\n") // Usa direttamente emailConfig.Body

            <-ticker.C // Rispetta il rate limit
            fmt.Printf("Tentativo di invio email a %s...\n", email)
            err = sendEmail(email, subject, body, smtpConfig["server"], smtpConfig["port"], smtpConfig["user"], smtpConfig["password"])
            if err != nil {
                fmt.Fprintf(os.Stderr, "Errore durante l'invio dell'email a %s: %v\n", email, err)
                updateSendMailLog(logPath, email, fmt.Sprintf("Errore: %v", err))
            } else {
                fmt.Printf("Email inviata a %s\n", email)
                updateSendMailLog(logPath, email, "Inviata")
            }
        }
    }
}

func processEmails(ctx context.Context, emailCSV, logPath string, smtpConfig map[string]string) error {
	fmt.Println("Inizio processo di invio email...")
	file, err := os.Open(emailCSV)
	if err != nil {
		return fmt.Errorf("impossibile aprire il file delle email: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	_, err = reader.Read() // Salta l'intestazione
	if err != nil {
		return fmt.Errorf("errore durante la lettura dell'intestazione del file delle email: %v", err)
	}

	emailQueue := make(chan []string, 100)
	wg := &sync.WaitGroup{}

	// Lancia più worker per inviare le email
	numWorkers := 5
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go sendEmailsFromQueue(ctx, emailQueue, smtpConfig, logPath, "email_config.json", wg)
	}

	// Leggi le email dal CSV e mettile in coda
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) < 2 {
			continue
		}
		emailQueue <- record
	}

	close(emailQueue)
	wg.Wait()
	return nil
}

func generateSQL(csvFilePath, outputSQLPath, tableName string, columns []string, fixedCategoryValue string) error {
	file, err := os.Open(csvFilePath)
	if err != nil {
		return fmt.Errorf("impossibile aprire il file CSV: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ','

	headers, err := reader.Read()
	if err != nil {
		return fmt.Errorf("errore nella lettura dell'intestazione CSV: %v", err)
	}

	rows := [][]string{}
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("errore nella lettura del file CSV: %v", err)
		}

		if len(record) != len(headers) {
			continue
		}

		cleanedRow := []string{}
		for i, value := range record {
			cleanedValue := strings.ReplaceAll(value, "'", " ")
			cleanedValue = strings.TrimRight(cleanedValue, "/")
			cleanedValue = strings.TrimSpace(cleanedValue)

			if i == 1 {
				cleanedRow = append(cleanedRow, fmt.Sprintf("''%s''", fixedCategoryValue))
			} else {
				cleanedRow = append(cleanedRow, fmt.Sprintf("''%s''", cleanedValue))
			}
		}
		rows = append(rows, cleanedRow)
	}

	outputFile, err := os.Create(outputSQLPath)
	if err != nil {
		return fmt.Errorf("errore nella creazione del file SQL: %v", err)
	}
	defer outputFile.Close()

	writer := bufio.NewWriter(outputFile)
	defer writer.Flush()

	_, err = writer.WriteString("BEGIN\n")
	if err != nil {
		return err
	}

	for _, row := range rows {
		values := strings.Join(row, ", ")

		sqlStatement := fmt.Sprintf(
			"    EXECUTE IMMEDIATE 'INSERT INTO %s (%s) VALUES (%s)';\n",
			tableName,
			strings.Join(columns, ", "),
			values,
		)
		_, err = writer.WriteString(sqlStatement)
		if err != nil {
			return err
		}
	}

	_, err = writer.WriteString("END;\n")
	return err
}

func createKeywordJobs(langCode, keywordFile, comuniFile string, maxDepth int, email bool) ([]*gmaps.GmapJob, error) {
	var keywordJobs []*gmaps.GmapJob

	keywords, err := readCSVColumn(keywordFile, "keyword")
	if err != nil {
		return nil, err
	}

	comuni, err := readCSVColumn(comuniFile, "comuni")
	if err != nil {
		return nil, err
	}

	for _, keyword := range keywords {
		for _, comune := range comuni {
			query := keyword + " " + comune
			job := gmaps.NewGmapJob("", langCode, query, maxDepth, email)
			keywordJobs = append(keywordJobs, job)
		}
	}

	return keywordJobs, nil
}

func convertToJobs(keywordJobs []*gmaps.GmapJob) []scrapemate.IJob {
	var jobs []scrapemate.IJob
	for _, job := range keywordJobs {
		jobs = append(jobs, job)
	}
	return jobs
}

func NewCustomCsvWriter(w *csv.Writer) scrapemate.ResultWriter {
	return &customCsvWriter{writer: w}
}

type customCsvWriter struct {
	writer *csv.Writer
}

func (cw *customCsvWriter) WriteResult(result scrapemate.Result) error {
	entry, ok := result.Data.(*gmaps.Entry)
	if !ok {
		return fmt.Errorf("tipo di dato non valido per il risultato")
	}

	record := []string{
		entry.Title,
		entry.Category,
		entry.WebSite,
		entry.Phone,
		entry.Street,
		entry.City,
		entry.Province,
		entry.Email,
		entry.Protocol,
		entry.Technology,
		entry.CookieBanner, // Aggiungi Cookie Banner
	}

	// Log della riga generata
	fmt.Printf("Riga generata: %v\n", record)

	// Convalida il numero di campi rispetto all'intestazione
	expectedFields := 11 // Aggiornato per includere Cookie Banner
	if len(record) != expectedFields {
		fmt.Printf("Riga scartata (numero di campi errato): %v\n", record)
		return nil
	}

	return cw.writer.Write(record)
}

func (cw *customCsvWriter) Run(ctx context.Context, results <-chan scrapemate.Result) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case result, ok := <-results:
			if !ok {
				return nil
			}
			if err := cw.WriteResult(result); err != nil {
				return err
			}
		}
	}
}

func readCSVColumn(filePath, columnName string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}

	columnIndex := -1
	for i, col := range header {
		normalizedHeader := strings.ToLower(strings.TrimSpace(col))
		if normalizedHeader == strings.ToLower(strings.TrimSpace(columnName)) {
			columnIndex = i
			break
		}
	}

	if columnIndex == -1 {
		return nil, fmt.Errorf("colonna %s non trovata", columnName)
	}

	var values []string
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if columnIndex < len(record) {
			value := strings.TrimSpace(record[columnIndex])
			if value != "" {
				values = append(values, value)
			}
		}
	}

	return values, nil
}
