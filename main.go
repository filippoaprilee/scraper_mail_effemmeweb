package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/smtp"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/gosom/google-maps-scraper/gmaps"
	"github.com/gosom/scrapemate"
	"github.com/gosom/scrapemate/scrapemateapp"
)

type EmailConfig struct {
    Templates map[string]struct {
        Subject string   `json:"subject"`
        Body    []string `json:"body"`
    } `json:"templates"`
}

// Funzione per cancellare il terminale
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

// Funzione per ottenere la larghezza del terminale
func getTerminalWidth() int {
    cmd := exec.Command("tput", "cols") // Comando per ottenere la larghezza del terminale
    cmd.Stdin = os.Stdin
    out, err := cmd.Output()
    if err != nil {
        return 80 // Larghezza di default se il comando fallisce
    }
    width, err := strconv.Atoi(strings.TrimSpace(string(out))) // Converte l'output in un intero
    if err != nil {
        return 80
    }
    return width
}

// Funzione per centrare il testo nel terminale
func centerText(text string, terminalWidth int) string {
    lines := strings.Split(text, "\n")
    var centeredLines []string
    for _, line := range lines {
        padding := (terminalWidth - len(line)) / 2
        if padding > 0 {
            centeredLines = append(centeredLines, strings.Repeat(" ", padding)+line)
        } else {
            centeredLines = append(centeredLines, line)
        }
    }
    return strings.Join(centeredLines, "\n")
}


// Funzione per stampare l'uso dell'utility
func printUsage() {
    banner := `
======================================================================================================================
			🚀 EFFEMMEWEB UTILITY 🚀
======================================================================================================================
		███████╗███████╗███████╗███████╗███╗   ███╗███╗   ███╗███████╗██╗    ██╗███████╗██████╗ 
		██╔════╝██╔════╝██╔════╝██╔════╝████╗ ████║████╗ ████║██╔════╝██║    ██║██╔════╝██╔══██╗
		█████╗  █████╗  █████╗  █████╗  ██╔████╔██║██╔████╔██║█████╗  ██║ █╗ ██║█████╗  ██████╔╝
		██╔══╝  ██╔══╝  ██╔══╝  ██╔══╝  ██║╚██╔╝██║██║╚██╔╝██║██╔══╝  ██║███╗██║██╔══╝  ██╔══██╗
		███████╗██║     ██║     ███████╗██║ ╚═╝ ██║██║ ╚═╝ ██║███████╗╚███╔███╔╝███████╗██████╔╝
		╚══════╝╚═╝     ╚═╝     ╚══════╝╚═╝     ╚═╝╚═╝     ╚═╝╚══════╝ ╚══╝╚══╝ ╚══════╝╚═════╝ 
																					
======================================================================================================================
    
`
    terminalWidth := getTerminalWidth()
    centeredBanner := centerText(banner, terminalWidth)

    title := color.New(color.FgCyan, color.Bold).SprintFunc()
    section := color.New(color.FgGreen, color.Bold).SprintFunc()
    highlight := color.New(color.FgYellow, color.Bold).SprintFunc()
    important := color.New(color.FgRed, color.Bold).SprintFunc()

    // Mostra il banner centrato
    fmt.Println(title(centeredBanner)) 
    fmt.Println(title("Benvenuto in EFFEMMEWEB Utility, il tuo strumento tuttofare!\n"))

    fmt.Println(section("📘 COME SI USA:"))
    fmt.Println("1️⃣  Assicurati che i seguenti file di configurazione siano nella directory principale:")
    fmt.Printf("   - %s: %s\n", highlight("keyword.csv"), "Parole chiave per lo scraping.")
    fmt.Printf("   - %s: %s\n", highlight("comuni.csv"), "Elenco dei comuni per combinazioni di ricerca.")
    fmt.Printf("   - %s: %s\n", highlight("email_config.json"), "Configurazione email (oggetto e corpo).\n")

    fmt.Println(section("⚙️  FUNZIONALITÀ DISPONIBILI:"))
    fmt.Printf("   ➤ %s: Genera un file CSV con i risultati dello scraping da Google Maps.\n", highlight("Scraping"))
    fmt.Println("       📌 Dettagli inclusi: Nome Attività, Categoria, Sito Web, Telefono, ecc.")
    fmt.Printf("   ➤ %s: Converte i risultati del CSV in istruzioni SQL per il database.\n", highlight("Generazione SQL"))
    fmt.Printf("   ➤ %s: Converte i risultati del CSV in un file email per inviarle ai destinatari.\n", highlight("Generazione Email"))
    fmt.Printf("   ➤ %s: Invia email personalizzate utilizzando i destinatari da un file CSV.\n", highlight("Invio Email"))

    fmt.Println(section("🔔 NOTE IMPORTANTI:"))
    fmt.Printf("   - %s\n", important("Puoi interrompere l'esecuzione in sicurezza usando CTRL+C."))
    fmt.Println("   - Assicurati di avere una connessione a internet per utilizzare scraping e invio email.")

    fmt.Println(title(centeredBanner)) // Mostra il banner centrato anche alla fine
}

// Funzione per stampare un timer elegante
func printTimer(minutes int) {
    fmt.Println("\n⏳ Tempo di esecuzione: ", color.New(color.FgGreen).Sprintf("%d minuti", minutes))
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

    if len(config.Templates) == 0 {
        return EmailConfig{}, fmt.Errorf("il file di configurazione email è vuoto o incompleto")
    }

    return config, nil
}

func getEmailTemplate(config EmailConfig, siteExists bool, protocol string, seoScore float64, cookieBanner string) (string, string, error) {
    var templateKey string

    if !siteExists {
        templateKey = "site_not_found"
    } else if protocol == "http" {
        templateKey = "site_http"
    } else if seoScore < 75 {
        templateKey = "seo_score_low"
    } else if cookieBanner == "missing" {
        templateKey = "cookie_banner_missing"
    } else {
        templateKey = "default"  // Impostiamo un template di default se nessuna delle condizioni precedenti è soddisfatta
    }

    template, exists := config.Templates[templateKey]
    if !exists {
        return "", "", fmt.Errorf("template non trovato per la chiave: %s", templateKey)
    }

    subject := template.Subject
    body := strings.Join(template.Body, "\n")
    return subject, body, nil
}

func main() {
	clearTerminal()

	printUsage()

	// Variabile per sapere se un file CSV è stato generato
	var generatedCSV string

	// Gestione del contesto per il programma
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Gestione del segnale CTRL+C
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Estrazione delle categorie
    categories, err := extractCategoriesFromKeywordFile("./keyword.csv")
    if err != nil {
        fmt.Println("Errore nell'estrazione delle categorie:", err)
        return
    }

	go func() {
        <-signalChan
        fmt.Println("\nCTRL+C rilevato. Attendi...")
        cancel()

        // Controlla se un file CSV è stato generato
        generatedCSV, err := findLastGeneratedCSV()
        if err != nil {
            fmt.Println(color.New(color.FgRed).Sprint("Errore durante la ricerca del file CSV: ", err))
            return
        }

        if generatedCSV != "" {
            // Elabora il file CSV senza richiedere una nuova selezione della categoria
            handleCSVOptions(ctx, generatedCSV)
        } else {
            fmt.Println(color.New(color.FgRed).Sprint("Nessun file CSV generato. Uscita dal programma."))
        }

        os.Exit(0)
    }()

	// Menu principale
	for {
        fmt.Println("\nSeleziona un'opzione dal menu:")
        fmt.Println("1. 🕵️‍♂️ Avvia lo scraping per raccogliere dati da Google Maps (Richiede connessione Internet).")
        fmt.Println("2. 💾 Converti un file CSV esistente in istruzioni SQL.")
        fmt.Println("3. 📧 Genera un file email da un CSV.")
        fmt.Println("4. 📤 Invia email utilizzando un file CSV.")
        fmt.Println("5. ❌ Esci dall'applicazione.")
        fmt.Print("\n" + color.New(color.FgYellow).Sprint("Scegli un'opzione (1-5): "))

        reader := bufio.NewReader(os.Stdin)
        choice, _ := reader.ReadString('\n')
        choice = strings.TrimSpace(choice)

        switch choice {
        case "1":
            if confirmAction("Confermi di voler avviare il processo di scraping? Inserisci 'y' per continuare o 'n' per annullare:") {
                if err := runScrapingFlow(ctx, categories, &generatedCSV); err != nil {
                    fmt.Println(color.New(color.FgRed).Sprintf("Errore durante lo scraping: %v", err))
                }
            }
        case "2":
            csvFile := getExistingCSVFile()
            if csvFile != "" {
                // Inizia direttamente la generazione del file SQL per il CSV selezionato
                fmt.Printf("Generazione file SQL per il CSV: %s\n", csvFile)
                
                // Genera il file SQL senza ulteriori richieste
                category := extractCategoryFromFileName(csvFile) // Estrai la categoria dal nome del file CSV
                if err := generateSQLFromCSV(ctx, csvFile, category); err != nil {
                    fmt.Println(color.New(color.FgRed).Sprintf("Errore durante la generazione del file SQL: %v", err))
                } else {
                    fmt.Println("File SQL generato con successo.")
                }
            }
        case "3":
            csvFile := getExistingCSVFile()
            if csvFile != "" {
                // Inizia direttamente la generazione del file email per il CSV selezionato
                fmt.Printf("Generazione file email per il CSV: %s\n", csvFile)
        
                // Genera il file email senza ulteriori richieste
                category := extractCategoryFromFileName(csvFile) // Estrai la categoria dal nome del file CSV
                if err := generateEmailsToSend(csvFile, category); err != nil {
                    fmt.Println(color.New(color.FgRed).Sprintf("Errore durante la generazione del file email: %v", err))
                } else {
                    fmt.Println("File email generato con successo.")
                }
            }
        case "4":
            csvFile := getExistingCSVFile()
            if csvFile != "" {
                // Inizia direttamente l'invio delle email per il CSV selezionato
                fmt.Printf("Invio email per il CSV: %s\n", csvFile)
                
                // Passa il file CSV per l'invio delle email
                smtpConfig := map[string]string{
                    "server":   "mail.effemmeweb.it",
                    "port":     "465",
                    "user":     "info@effemmeweb.it",
                    "password": "Ludovica2021", // Assicurati di usare la password corretta
                }
        
                if err := processEmails(ctx, csvFile, "sendmaillog.csv", smtpConfig); err != nil {
                    fmt.Println(color.New(color.FgRed).Sprintf("Errore durante l'invio delle email: %v", err))
                } else {
                    fmt.Println("Email inviate con successo.")
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

func getExistingCSVFile() string {
	files, err := filepath.Glob("csv_results/*.csv")
	if err != nil || len(files) == 0 {
		fmt.Println(color.New(color.FgRed).Sprint("Nessun file CSV trovato nella directory 'csv_results'."))
		return ""
	}

	fmt.Println("File CSV disponibili:")
	for i, file := range files {
		fmt.Printf("%d. %s\n", i+1, file)
	}

	fmt.Print("Seleziona un file CSV (inserisci il numero): ")
	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	index, err := strconv.Atoi(choice)
	if err != nil || index < 1 || index > len(files) {
		fmt.Println(color.New(color.FgRed).Sprint("Scelta non valida."))
		return ""
	}

	return files[index-1]
}

// Funzione per confermare un'azione
func confirmAction(message string) bool {
	fmt.Printf("%s (y/n): ", message)
	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(strings.ToLower(choice))
	return choice == "y"
}

func runScrapingFlow(ctx context.Context, categories []string, generatedCSV *string) error {
    fmt.Println("\n📋 Ecco la lista delle categorie disponibili per lo scraping:")
	for i, category := range categories {
		fmt.Printf("   %d. %s\n", i+1, category)
	}

	if !confirmAction("Vuoi procedere con lo scraping per tutte queste categorie?") {
		fmt.Println("Scraping annullato.")
		return nil
	}

	startTime := time.Now()
    fmt.Println("Avvio dello scraping...")

    // Canale per raccogliere i file CSV generati da tutte le categorie
    resultChannel := make(chan string, len(categories))

    var wg sync.WaitGroup // Gruppo di attesa per le goroutine

    // Avvia lo scraping per ogni categoria contemporaneamente
    for _, category := range categories {
        wg.Add(1) // Aggiungi un lavoro al gruppo di attesa

        // Avvia una goroutine per lo scraping della categoria
        go func(category string) {
            defer wg.Done() // Segna la goroutine come terminata

            newCSV, err := runScrapingForCategory(ctx, category)
            if err != nil {
                fmt.Println(color.New(color.FgRed).Sprintf("Errore durante lo scraping per la categoria %s: %v", category, err))
                return
            }

            // Invia il file generato al canale
            resultChannel <- newCSV
        }(category)
    }

    // Attendi che tutte le goroutine terminino
    wg.Wait()
    close(resultChannel) // Chiudi il canale

    // Raccogli i file generati e aggiorna la variabile generatedCSV
    for newCSV := range resultChannel {
        fmt.Printf("Nuovo file CSV generato: %s\n", newCSV)
        *generatedCSV = newCSV // Memorizza l'ultimo file generato
    }

    // Calcola il tempo trascorso in minuti
    elapsedTime := time.Since(startTime).Minutes()
    printTimer(int(elapsedTime))  // Mostra il tempo in minuti

    return nil
}

func processCSV(ctx context.Context, csvFile string, category string) error {
	// Gestione delle domande per il file CSV
	if askUser("Vuoi generare il file SQL? (y/n): ") {
		if err := generateSQLFromCSV(ctx, csvFile, category); err != nil {
			fmt.Println(color.New(color.FgRed).Sprintf("Errore durante la generazione del file SQL: %v", err))
		}
		fmt.Println("File SQL generato con successo.")
	}

	if askUser("Vuoi generare il file email? (y/n): ") {
		if err := generateEmailsToSend(csvFile, category); err != nil {
			return fmt.Errorf("Errore durante la generazione del file email: %v", err)
		}
		fmt.Println("File email generato con successo.")
	}

	if askUser("Vuoi inviare le email dall'account configurato? (y/n): ") {
		emailCSVPath := fmt.Sprintf("email_results/emails_to_send_Categoria_%s.csv", time.Now().Format("20060102_150405"))
        logPath := "sendmaillog.csv" // Usa logPath al posto di Path
		smtpConfig := map[string]string{
			"server":   "mail.effemmeweb.it",
			"port":     "465",
			"user":     "info@effemmeweb.it",
			"password": "Ludovica2021",
		}
		if err := processEmails(ctx, emailCSVPath, logPath, smtpConfig); err != nil {
            fmt.Println("Errore durante l'invio delle email:", err)
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

	// Recupera la categoria, ad esempio dal nome del file CSV
	// In questo caso supponiamo che la categoria possa essere estratta dal nome del file (ad esempio, "categoria_nomefile.csv")
	category := extractCategoryFromFileName(csvPath)

	// Procedi con le operazioni sul CSV
	if err := processCSV(ctx, csvPath, category); err != nil {
		fmt.Fprintf(os.Stderr, "Errore durante la gestione del CSV: %v\n", err)
	}
}

// Funzione per estrarre la categoria dal nome del file CSV
func extractCategoryFromFileName(filePath string) string {
	// Esegui una logica per ottenere la categoria, ad esempio
	// puoi usare una parte del nome del file o leggere dal contenuto del file stesso.
	// Ad esempio, se il nome del file è "categoria_nomefile.csv", puoi fare:
	parts := strings.Split(filepath.Base(filePath), "_")
	if len(parts) > 0 {
		return parts[0]  // Restituisce la prima parte come categoria
	}
	return "defaultCategory"  // Categoria di default se non trovata
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

func extractCategoriesFromKeywordFile(filePath string) ([]string, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return nil, fmt.Errorf("impossibile aprire il file: %v", err)
    }
    defer file.Close()

    reader := csv.NewReader(file)
    reader.Comma = ';'

    headers, err := reader.Read()
    if err != nil {
        return nil, fmt.Errorf("errore nella lettura dell'intestazione: %v", err)
    }

    // Supponiamo che la colonna "keyword" contenga le categorie
    columnIndex := -1
    for i, col := range headers {
        if strings.ToLower(strings.TrimSpace(col)) == "keyword" {
            columnIndex = i
            break
        }
    }

    if columnIndex == -1 {
        return nil, fmt.Errorf("colonna 'keyword' non trovata nel file %s", filePath)
    }

    var categories []string
    for {
        record, err := reader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, fmt.Errorf("errore nella lettura del file CSV: %v", err)
        }

        if len(record) > columnIndex {
            category := strings.TrimSpace(record[columnIndex])
            if category != "" {
                categories = append(categories, category)
            }
        }
    }

    if len(categories) == 0 {
        return nil, fmt.Errorf("nessuna categoria trovata nel file")
    }

    return categories, nil
}

// Funzione per filtrare le parole chiave per una determinata categoria
func filterKeywordsByCategory(keywords []string, category string) []string {
    var filteredKeywords []string

    // Leggi le parole chiave e filtrale per la categoria
    for _, keyword := range keywords {
        // Aggiungi la parola chiave se appartiene alla categoria
        if strings.Contains(keyword, category) {
            filteredKeywords = append(filteredKeywords, keyword)
        }
    }

    return filteredKeywords
}

// Definizione della directory principale
const baseDir = "scraper_results"

func runScrapingForCategory(ctx context.Context, category string) (string, error) {
    // Creazione della directory per i CSV
    csvDir := filepath.Join(baseDir, "csv_results")
    if err := os.MkdirAll(csvDir, os.ModePerm); err != nil {
        return "", fmt.Errorf("errore nella creazione della directory %s: %v", csvDir, err)
    }

    // Creazione della directory per i VCF
    vcfDir := filepath.Join(baseDir, "vcf_results")
    if err := os.MkdirAll(vcfDir, os.ModePerm); err != nil {
        return "", fmt.Errorf("errore nella creazione della directory %s: %v", vcfDir, err)
    }

    // Usa la categoria per creare un nome unico per il file CSV
    currentTime := time.Now().Format("20060102_150405")
    outputFileName := fmt.Sprintf("%s/%s_%s.csv", csvDir, category, currentTime)

    output, err := os.Create(outputFileName)
    if err != nil {
        return "", fmt.Errorf("errore durante la creazione del file CSV per la categoria %s: %v", category, err)
    }
    defer output.Close()

    csvWriter := csv.NewWriter(output)
    csvWriter.UseCRLF = true // Se vuoi terminatori di riga Windows-style
    defer csvWriter.Flush()

    // Scrivi l'intestazione del CSV
    err = csvWriter.Write([]string{
        "Nome Attività", "Categoria", "Sito Web", "Telefono", "Indirizzo", "Comune", "Provincia", "Email",
        "Protocollo", "Tecnologia", "Cookie Banner", "Hosting Provider", "Mobile Performance", "Desktop Performance", "Punteggio SEO",
        "Disponibilità Sito", "Stato Manutenzione", // Aggiunte nuove colonne
    })
    if err != nil {
        return "", fmt.Errorf("errore durante la scrittura dell'intestazione nel file CSV: %v", err)
    }

    // File VCF
    vcfFileName := fmt.Sprintf("%s/numeriditelefono_%s_%s.vcf", vcfDir, category, currentTime)
    vcfFile, err := os.Create(vcfFileName)
    if err != nil {
        return "", fmt.Errorf("errore durante la creazione del file VCF per la categoria %s: %v", category, err)
    }
    defer vcfFile.Close()

    // Configura lo scraping per la categoria specifica
    writers := []scrapemate.ResultWriter{
        NewCustomCsvWriterWithVCF(csvWriter, vcfFile),
    }

    opts := []func(*scrapemateapp.Config) error{
        scrapemateapp.WithConcurrency(8),
        scrapemateapp.WithExitOnInactivity(3 * time.Minute),
        scrapemateapp.WithJS(scrapemateapp.DisableImages()),
    }

    cfg, err := scrapemateapp.NewConfig(writers, opts...)
    if err != nil {
        return "", fmt.Errorf("errore durante la configurazione dello scraping per la categoria %s: %v", category, err)
    }

    app, err := scrapemateapp.NewScrapeMateApp(cfg)
    if err != nil {
        return "", fmt.Errorf("errore durante l'inizializzazione dello scraping per la categoria %s: %v", category, err)
    }

    // Ottieni le parole chiave per la categoria
    keywords, err := readCSVColumn("./keyword.csv", "keyword")
    if err != nil {
        return "", fmt.Errorf("errore durante la lettura delle keyword: %v", err)
    }

    // Filtro per la categoria corrente
    filteredKeywords := filterKeywordsByCategory(keywords, category)

    // Creazione dei lavori di scraping per la categoria
    keywordJobs, err := createKeywordJobs("it", filteredKeywords, "./comuni.csv", 10, true)
    if err != nil {
        return "", fmt.Errorf("errore durante la creazione dei lavori di scraping per la categoria %s: %v", category, err)
    }

    if len(keywordJobs) == 0 {
        return "", fmt.Errorf("nessun lavoro di scraping creato per la categoria %s", category)
    }

    jobs := convertToJobs(keywordJobs)

    // Avvia lo scraping con il contesto
    select {
    case <-ctx.Done():
        fmt.Println("Scraping interrotto.")
        return "", nil
    default:
        if err := app.Start(ctx, jobs...); err != nil && ctx.Err() != context.Canceled {
            return "", fmt.Errorf("errore durante lo scraping per la categoria %s: %v", category, err)
        }
    }

    return outputFileName, nil // Restituisce il nome del file CSV generato per la categoria
}

func cleanLastRowFromCSV(filePath string) error {
	// Apri il file per la lettura
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("impossibile aprire il file CSV: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	var rows [][]string

	// Leggi tutte le righe dal CSV
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Se si verifica un errore di lettura, lo logghiamo ma continuiamo
			fmt.Printf("Errore durante la lettura del CSV: %v\n", err)
			continue
		}
		rows = append(rows, record)
	}

	// Se il file ha più di una riga, rimuoviamo l'ultima
	if len(rows) > 1 {
		// Verifica che l'ultima riga abbia il numero corretto di colonne
		expectedColumns := len(rows[0])
		lastRow := rows[len(rows)-1]
		if len(lastRow) != expectedColumns {
			fmt.Println("Ultima riga con numero di campi errato, la rimuovo.")
			rows = rows[:len(rows)-1]
		}
	} else if len(rows) == 1 && len(rows[0]) == 0 {
		// Se il CSV ha solo una riga vuota, rimuovila
		rows = nil
	}

	// Scrivi il CSV aggiornato
	tempFilePath := filePath + ".tmp"
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		return fmt.Errorf("errore durante la creazione del file temporaneo: %v", err)
	}
	defer tempFile.Close()

	// Inizializza un writer CSV
    writer := csv.NewWriter(tempFile)
    defer writer.Flush()

    // Scrivi il CSV pulito
    if err := writer.WriteAll(rows); err != nil {
        return fmt.Errorf("errore durante la scrittura nel file CSV: %v", err)
    }

	// Sostituisci il file originale con il file temporaneo
	if err := os.Rename(tempFilePath, filePath); err != nil {
		return fmt.Errorf("errore durante la sostituzione del file CSV: %v", err)
	}

	return nil
}

func handleCSVOptions(ctx context.Context, csvFile string) {
    // Recupera la categoria, ad esempio dal nome del file CSV
    category := extractCategoryFromFileName(csvFile) // Estrai la categoria dal nome del file

    fmt.Printf("\nCosa vuoi fare con il file CSV generato (%s)?\n", csvFile)
    fmt.Println("1. Generare file SQL")
    fmt.Println("2. Generare file email")
    fmt.Println("3. Inviare email")
    fmt.Println("4. Uscire")
    fmt.Print("Scegli un'opzione (1-4): ")

    reader := bufio.NewReader(os.Stdin)
    choice, _ := reader.ReadString('\n')
    choice = strings.TrimSpace(choice)

    switch choice {
    case "1":
        fmt.Println(color.New(color.FgGreen).Sprint("Generazione file SQL..."))
        if err := generateSQLFromCSV(ctx, csvFile, category); err != nil {
            fmt.Println(color.New(color.FgRed).Sprintf("Errore durante la generazione del file SQL: %v", err))
        }
    case "2":
        fmt.Println(color.New(color.FgGreen).Sprint("Generazione file email..."))
        if err := generateEmailsToSend(csvFile, category); err != nil {
            fmt.Println(color.New(color.FgRed).Sprintf("Errore durante la generazione del file email: %v", err))
        }
    case "3":
        fmt.Println(color.New(color.FgGreen).Sprint("Invio email in corso..."))
        smtpConfig := map[string]string{
            "server":   "mail.effemmeweb.it",
            "port":     "465",
            "user":     "info@effemmeweb.it",
            "password": "Ludovica2021",
        }
        if err := processEmails(ctx, csvFile, "sendmaillog.csv", smtpConfig); err != nil {
            fmt.Println(color.New(color.FgRed).Sprintf("Errore durante l'invio delle email: %v", err))
        }
    case "4":
        fmt.Println(color.New(color.FgGreen).Sprint("Uscita. Arrivederci!"))
        return
    default:
        fmt.Println(color.New(color.FgRed).Sprint("Opzione non valida. Riprova."))
    }
}

func generateSQLFromCSV(ctx context.Context, csvFilePath string, category string) error {
	// Pulizia dell'ultima riga del CSV (se necessario)
    if err := cleanLastRowFromCSV(csvFilePath); err != nil {
		return fmt.Errorf("errore durante la pulizia dell'ultima riga del CSV: %v", err)
	}

	sqlDir := filepath.Join(baseDir, "sql_results")
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
		"EFFEMMEWEB_PROTOCOLLO",    // Aggiunta colonna Protocollo
		"EFFEMMEWEB_TECNOLOGIA",    // Aggiunta colonna Tecnologia
		"EFFEMMEWEB_COOKIE_POLICY", // Aggiunta colonna Cookie Banner
		"EFFEMMEWEB_HOSTING_PROVIDER", // Aggiunta Hosting Provider
		"EFFEMMEWEB_MOBILE_PERFORMANCE", // Aggiunta Mobile Performance
		"EFFEMMEWEB_DESKTOP_PERFORMANCE", // Aggiunta Desktop Performance
		"EFFEMMEWEB_SEO_SCORE", // Aggiunta Seo Score
        "EFFEMMEWEB_DISPONIBILITA_SITO",
        "EFFEMMEWEB_STATO_MANUTENZIONE",
	}

	// Usa la categoria passata come parametro
	fixedCategoryValue := category

	currentTime := time.Now().Format("20060102_150405")
	outputSQLPath := fmt.Sprintf("%s/output_%s_%s.sql", sqlDir, fixedCategoryValue, currentTime)

	// Genera il file SQL per questa categoria
	err := generateSQL(csvFilePath, outputSQLPath, tableName, columns, fixedCategoryValue)
	if err != nil {
		return fmt.Errorf("errore durante la generazione del file SQL: %v", err)
	}

	fmt.Printf("File SQL generato con successo: %s\n", outputSQLPath)

	return nil
}

func generateEmailsToSend(csvFilePath string, category string) error {
    emailDir := filepath.Join(baseDir, "email_results")
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

    // Crea il file CSV di output per le email
    outputFile, err := os.Create(outputEmailFilePath)
    if err != nil {
        return fmt.Errorf("errore nella creazione del file email CSV: %v", err)
    }
    defer outputFile.Close()

    writer := csv.NewWriter(outputFile)
    writer.UseCRLF = true // Se vuoi terminatori di riga Windows-style
    defer writer.Flush()

    // Scrive l'intestazione
    writer.Write([]string{"Email", "Nome Attività"})

    // Legge e scrive le righe dal file di input
    for {
        record, err := reader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            return fmt.Errorf("errore nella lettura del file CSV: %v", err)
        }

        // Assicurati che ci siano abbastanza colonne
        if len(record) > emailIndex && len(record) > nameIndex {
            email := strings.TrimSpace(record[emailIndex])
            name := strings.TrimSpace(record[nameIndex])

            // Scrive una riga nel file CSV di output
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

func sendCustomEmail(to, name, emailConfigPath, smtpServer, smtpPort, smtpUser, smtpPassword string, siteExists bool, protocol string, seoScore float64, cookieBanner string) error {
    // Carica la configurazione dei template
    config, err := loadEmailConfig(emailConfigPath)
    if err != nil {
        return fmt.Errorf("errore nel caricamento della configurazione: %v", err)
    }

    // Seleziona il template giusto in base ai parametri
    subject, body, err := getEmailTemplate(config, siteExists, protocol, seoScore, cookieBanner)
    if err != nil {
        return fmt.Errorf("errore nella selezione del template: %v", err)
    }

    // Personalizza il corpo dell'email sostituendo il placeholder {name}
    body = strings.ReplaceAll(body, "{name}", name)

    // Invia l'email
    return sendEmail(to, subject, body, smtpServer, smtpPort, smtpUser, smtpPassword)
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

func updateSendMailLog(filePath, name, email, status, template string) error {
    file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return fmt.Errorf("errore durante l'apertura del file di log: %v", err)
    }
    defer file.Close()

    writer := csv.NewWriter(file)
    defer writer.Flush()

    // Se il file è vuoto, scriviamo l'intestazione
    stat, err := file.Stat()
    if err != nil {
        return fmt.Errorf("errore durante il recupero delle informazioni del file: %v", err)
    }

    if stat.Size() == 0 {
        // Scriviamo l'intestazione solo se il file è vuoto
        err = writer.Write([]string{"Nome Attività", "Email", "Stato", "Template"})
        if err != nil {
            return fmt.Errorf("errore durante la scrittura dell'intestazione nel file di log: %v", err)
        }
    }

    // Scrivi la nuova riga di log
    err = writer.Write([]string{name, email, status, template})
    if err != nil {
        return fmt.Errorf("errore durante la scrittura nel file di log: %v", err)
    }

    return nil
}

func sendEmailsFromQueue(ctx context.Context, emailQueue <-chan []string, smtpConfig map[string]string, logPath string, emailConfigPath string, wg *sync.WaitGroup) {
    defer wg.Done()

    ticker := time.NewTicker(time.Second * 3600 / 100) // Rate limit: 100 email/ora
    defer ticker.Stop()

    // Carica la configurazione dei template
    emailConfig, err := loadEmailConfig(emailConfigPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Errore durante il caricamento della configurazione email: %v\n", err)
        return
    }

    // Leggi il log delle email già inviate
    log, err := readSendMailLog(logPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Errore durante la lettura del log delle email: %v\n", err)
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
            site := emailData[2] // Supponiamo che il sito sia nella colonna 3 (indice 2)
            cookieBannerColumn := emailData[8] // Supponiamo che la colonna dei cookie sia nella colonna 9 (indice 8)

            fmt.Printf("Elaborazione email: %s - Nome: %s\n", email, name)

            // Verifica se l'email è già presente nel log con stato "Inviata"
            if status, exists := log[email]; exists && status == "Inviata" {
                fmt.Printf("Email già inviata: %s\n", email)
                continue // Salta questa email
            }

            // Determina se il sito esiste
            siteExists := (site != "")

            // Determina il protocollo
            var protocol string
            if strings.HasPrefix(site, "https://") {
                protocol = "https"
            } else {
                protocol = "http"
            }

            // Imposta il valore del cookie banner
            cookieBanner := "No"
            if cookieBannerColumn == "Sì" {
                cookieBanner = "Yes"
            }

            // Calcola il punteggio SEO (logica fittizia per esempio)
            seoScore := 80.0 // Puoi aggiungere logica per calcolare il vero punteggio SEO

            // Seleziona il template giusto in base ai parametri
            subject, body, err := getEmailTemplate(emailConfig, siteExists, protocol, seoScore, cookieBanner)
            if err != nil {
                fmt.Fprintf(os.Stderr, "Errore nella selezione del template: %v\n", err)
                continue
            }

            // Personalizza il subject e il body usando il template selezionato
            body = strings.ReplaceAll(body, "{name}", name)

            // Invia l'email
            err = sendEmail(email, subject, body, smtpConfig["server"], smtpConfig["port"], smtpConfig["user"], smtpConfig["password"])
            if err != nil {
                fmt.Fprintf(os.Stderr, "Errore durante l'invio dell'email a %s: %v\n", email, err)
                updateSendMailLog(logPath, name, email, fmt.Sprintf("Errore: %v", err), subject)
            } else {
                fmt.Printf("Email inviata a %s\n", email)
                updateSendMailLog(logPath, name, email, "Inviata", subject) // Aggiungi il template usato
            }

            <-ticker.C // Rispetta il rate limit
        }
    }
}

func processEmails(ctx context.Context, emailCSV, logPath string, smtpConfig map[string]string) error {
    startTime := time.Now()  // Inizia a tracciare il tempo
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

    // Calcola il tempo trascorso in minuti
    elapsedTime := time.Since(startTime).Minutes()
    printTimer(int(elapsedTime))  // Mostra il tempo in minuti

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
            // Gestione delle colonne con dati vuoti
            cleanedValue := strings.ReplaceAll(value, "'", " ")
            cleanedValue = strings.TrimRight(cleanedValue, "/")
            cleanedValue = strings.TrimSpace(cleanedValue)

            // Se la colonna è la categoria, usa il valore fisso
            if i == 1 { // Supponiamo che la categoria sia nella seconda colonna (indice 1)
                cleanedRow = append(cleanedRow, fmt.Sprintf("''%s''", fixedCategoryValue))
            } else {
                cleanedRow = append(cleanedRow, fmt.Sprintf("''%s''", cleanedValue))
            }
        }

        // Aggiungi le nuove colonne per PROTOCOLLO, TECNOLOGIA, COOKIE_POLICY, ecc. solo se hanno valore
        protocol := "" // Aggiungi qui il valore per "protocol"
        technology := "" // Aggiungi qui il valore per "technology"
        cookiePolicy := "" // Aggiungi qui il valore per "cookiePolicy"
        hostingProvider := "" // Aggiungi qui il valore per "hostingProvider"
        mobilePerformance := "" // Aggiungi qui il valore per "mobilePerformance"
        desktopPerformance := "" // Aggiungi qui il valore per "desktopPerformance"
        seoScore := "" // Aggiungi qui il valore per "seoScore"
        siteAvailability := "" // Valore di disponibilità del sito
        siteMaintenance := ""  // Valore di stato manutenzione

        // Aggiungi le colonne solo se ci sono valori
        if protocol != "" {
            cleanedRow = append(cleanedRow, fmt.Sprintf("''%s''", protocol))
        }
        if technology != "" {
            cleanedRow = append(cleanedRow, fmt.Sprintf("''%s''", technology))
        }
        if cookiePolicy != "" {
            cleanedRow = append(cleanedRow, fmt.Sprintf("''%s''", cookiePolicy))
        }
        if hostingProvider != "" {
            cleanedRow = append(cleanedRow, fmt.Sprintf("''%s''", hostingProvider))
        }
        if mobilePerformance != "" {
            cleanedRow = append(cleanedRow, fmt.Sprintf("''%s''", mobilePerformance))
        }
        if desktopPerformance != "" {
            cleanedRow = append(cleanedRow, fmt.Sprintf("''%s''", desktopPerformance))
        }
        if seoScore != "" {
            cleanedRow = append(cleanedRow, fmt.Sprintf("''%s''", seoScore))
        }
        if siteAvailability != "" {
            cleanedRow = append(cleanedRow, fmt.Sprintf("''%s''", siteAvailability))
        }
        if siteMaintenance != "" {
            cleanedRow = append(cleanedRow, fmt.Sprintf("''%s''", siteMaintenance))
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

// Funzione per creare i lavori di scraping
func createKeywordJobs(langCode string, keywords []string, comuniFile string, maxDepth int, email bool) ([]*gmaps.GmapJob, error) {
    var keywordJobs []*gmaps.GmapJob

    // Leggi i comuni dal file
    comuni, err := readCSVColumn(comuniFile, "comuni")
    if err != nil {
        return nil, err
    }

    // Crea i lavori di scraping per ogni parola chiave
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

func writeVCF(vcfFile *os.File, name, phone string) error {
    // Rimuovi tutti gli spazi dal numero di telefono
    trimmedPhone := strings.ReplaceAll(phone, " ", "")
    
    vcfContact := fmt.Sprintf(
        "BEGIN:VCARD\nVERSION:3.0\nFN:%s\nTEL;TYPE=CELL:%s\nEND:VCARD\n",
        name,
        trimmedPhone,
    )
    _, err := vcfFile.WriteString(vcfContact)
    return err
}

func NewCustomCsvWriterWithVCF(w *csv.Writer, vcfFile *os.File) scrapemate.ResultWriter {
	return &customCsvWriter{
		writer:   w,
		vcfFile:  vcfFile,
		emails:   make(map[string]bool),
		phones:   make(map[string]bool),
		names:    make(map[string]bool),
	}
}

// Aggiorna la struttura per includere il file `.vcf`
type customCsvWriter struct {
	writer  *csv.Writer
	vcfFile *os.File
	emails  map[string]bool
	phones  map[string]bool
	names   map[string]bool
}

func (cw *customCsvWriter) WriteResult(result scrapemate.Result) error {
    entry, ok := result.Data.(*gmaps.Entry)
    if !ok {
        return fmt.Errorf("tipo di dato non valido per il risultato")
    }

    // Verifica se tutti i campi sono vuoti
    if entry.Title == "" && entry.Category == "" && entry.WebSite == "" && entry.Phone == "" &&
        entry.Street == "" && entry.City == "" && entry.Province == "" && entry.Email == "" &&
        entry.Protocol == "" && entry.Technology == "" && entry.CookieBanner == "" &&
        entry.HostingProvider == "" && entry.MobilePerformance == "" &&
        entry.DesktopPerformance == "" && entry.SeoScore == "" && entry.SiteAvailability == "" &&
        entry.SiteMaintenance == "" {
        fmt.Println("⚠️  Riga completamente vuota rilevata. Ignorata.")
        return nil // Non scrive la riga
    }

    email := entry.Email
    phone := entry.Phone
    name := entry.Title // Nome attività

    // Controllo duplicati
    if email != "" && cw.emails[email] {
        fmt.Printf("🔁 Duplicato trovato per l'email: %s. Riga ignorata.\n", email)
        return nil
    }
    if phone != "" && cw.phones[phone] {
        fmt.Printf("🔁 Duplicato trovato per il telefono: %s. Riga ignorata.\n", phone)
        return nil
    }
    if name != "" && cw.names[name] {
        fmt.Printf("🔁 Duplicato trovato per il nome dell'attività: %s. Riga ignorata.\n", name)
        return nil
    }

    // Aggiungi ai set dei duplicati
    if email != "" {
        cw.emails[email] = true
    }
    if phone != "" {
        cw.phones[phone] = true
    }
    if name != "" {
        cw.names[name] = true
    }

    // Prepara la riga CSV
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
        entry.CookieBanner,
        entry.HostingProvider,
        entry.MobilePerformance,
        entry.DesktopPerformance,
        entry.SeoScore,
        entry.SiteAvailability,
        entry.SiteMaintenance,
    }

    // Rimuovi virgolette doppie dai campi
    for i, field := range record {
        record[i] = strings.ReplaceAll(field, "\"", "")
    }

    // Verifica il numero di campi
    expectedFields := 17
    if len(record) != expectedFields {
        fmt.Printf("⚠️  Riga scartata (numero di campi errato): %v\n", record)
        return nil
    }

    // Scrivi il contatto nel file VCF
    if phone != "" {
        if err := writeVCF(cw.vcfFile, name, phone); err != nil {
            fmt.Printf("❌ Errore durante la scrittura del contatto VCF: %v\n", err)
        }
    }

    // Rimuove le virgolette doppie da ogni campo
    for i, field := range record {
        record[i] = strings.ReplaceAll(field, "\"", "")
    }

    // Scrivi la riga nel CSV
    if err := cw.writer.Write(record); err != nil {
        return fmt.Errorf("❌ Errore durante la scrittura nel file CSV: %v", err)
    }

    return nil
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

// Funzione per leggere una colonna da un file CSV
func readCSVColumn(filePath string, columnName string) ([]string, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    reader := csv.NewReader(file)
    reader.Comma = ';'

    // Leggi l'intestazione
    header, err := reader.Read()
    if err != nil {
        return nil, err
    }

    // Trova l'indice della colonna con il nome specificato
    columnIndex := -1
    for i, col := range header {
        if strings.ToLower(strings.TrimSpace(col)) == strings.ToLower(strings.TrimSpace(columnName)) {
            columnIndex = i
            break
        }
    }

    if columnIndex == -1 {
        return nil, fmt.Errorf("colonna '%s' non trovata", columnName)
    }

    // Leggi tutte le righe della colonna
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
            values = append(values, strings.TrimSpace(record[columnIndex]))
        }
    }

    return values, nil
}

