package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
    "math/rand"
	"io/ioutil"
	"net/smtp"
	"os"
	"os/exec"
    "regexp"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
    "net/url"
    "net/http"
	"bytes"
    "sort"
    "github.com/PuerkitoBio/goquery"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/gosom/google-maps-scraper/gmaps"
	"github.com/gosom/scrapemate"
	"github.com/gosom/scrapemate/scrapemateapp"
    "github.com/chromedp/chromedp"

)

type urlValues map[string]string

type EmailConfig struct {
    Templates map[string]struct {
        Subject string   `json:"subject"`
        Body    []string `json:"body"`
    } `json:"templates"`
}



// Metodo Encode per urlValues
func (u urlValues) Encode() string {
    var buf strings.Builder
    for key, value := range u {
        if buf.Len() > 0 {
            buf.WriteByte('&')
        }
        buf.WriteString(url.QueryEscape(key))
        buf.WriteByte('=')
        buf.WriteString(url.QueryEscape(value))
    }
    return buf.String()
}

// extractEmailsFromHTML estrae le email dal contenuto HTML analizzato
func extractEmailsFromHTML(htmlContent string) ([]string, error) {
	// Crea un documento goquery dal contenuto HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("errore nella creazione del documento goquery: %v", err)
	}

	// Trova l'elemento che contiene le email (ad esempio div.post-content)
	var emails []string
	doc.Find("div.post-content").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		// Dividi il testo in linee per individuare le email
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Usa una regex per verificare se la linea contiene un'email
			if isEmail(line) {
				emails = append(emails, line)
			}
		}
	})

	return emails, nil
}

// isEmail verifica se una stringa è un'email valida
func isEmail(s string) bool {
	re := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	return re.MatchString(s)
}

// readExistingEmails legge le email esistenti dal file CSV e le restituisce in un set.
func readExistingEmails(csvFilePath string) (map[string]struct{}, error) {
	existingEmails := make(map[string]struct{})

	file, err := os.Open(csvFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return existingEmails, nil // File non esiste ancora
		}
		return nil, fmt.Errorf("errore nell'apertura del file CSV esistente: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("errore nella lettura del file CSV esistente: %v", err)
		}

		if len(record) > 0 {
			email := strings.TrimSpace(record[0])
			existingEmails[email] = struct{}{}
		}
	}

	return existingEmails, nil
}

// appendEmailsToCSV aggiunge nuove email al file CSV esistente.
func appendEmailsToCSV(csvFilePath string, emails []string) error {


	// Rimuovi duplicati
	uniqueEmails := make(map[string]struct{})
	for _, email := range emails {
		uniqueEmails[email] = struct{}{}
	}

	// Trasforma le email in una lista e ordinale
	var emailList []string
	for email := range uniqueEmails {
		emailList = append(emailList, email)
	}
	sort.Strings(emailList)

	// Scrivi le email nel file CSV in un'unica riga separata da virgole
	csvContent := strings.Join(emailList, ",")
	return os.WriteFile(csvFilePath, []byte(csvContent), 0644)
}

func fetchPageContentWithChromedp(pageURL string) (string, error) {
	// Crea un nuovo contesto per Chrome in modalità incognito
	ctx, cancel := chromedp.NewExecAllocator(context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),       // Usa Chrome in modalità headless
			chromedp.Flag("disable-extensions", true),
			chromedp.Flag("incognito", true),      // Forza la modalità incognito
			chromedp.Flag("disable-cache", true),  // Disabilita la cache
		)...,
	)
	defer cancel()

	// Crea il contesto browser
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	// Crea una variabile per contenere il contenuto della pagina
	var htmlContent string

	// Esegui la navigazione su admin-ajax.php per effettuare il purge della cache
	purgeURL := "https://effemmeweb.it/wp-admin/admin-ajax.php?action=admin_bar_purge_cache&_wpnonce=309d1c5390"

	err := chromedp.Run(ctx,
		chromedp.Navigate(purgeURL),
		// Eventualmente attendi un minimo prima di proseguire
		chromedp.Sleep(2*time.Second),

		// Ora naviga verso la pagina desiderata
		chromedp.Navigate(pageURL),
		chromedp.OuterHTML("html", &htmlContent),
	)
	if err != nil {
		return "", fmt.Errorf("errore durante la navigazione con Chrome: %v", err)
	}

	return htmlContent, nil
}

// resetCSV elimina il file CSV se esiste
func resetCSV(csvFilePath string) error {
	err := os.Remove(csvFilePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("errore durante la rimozione del CSV: %v", err)
	}
	return nil
}

// updateUnsubscribeList aggiorna la lista delle email disiscritte.
func updateUnsubscribeList() error {
	pageURL := "https://effemmeweb.it/unsubscribe_list/"
	csvFilePath := "unsubscribe_list_api.csv"

	fmt.Println("🔄 Aggiornamento della lista delle email disiscritte...")

	// Recupera il contenuto della pagina usando Chromedp
	htmlContent, err := fetchPageContentWithChromedp(pageURL)
	if err != nil {
		return fmt.Errorf("errore durante il fetch: %v", err)
	}


	// Estrai le email dal contenuto HTML
	emails, err := extractEmailsFromHTML(htmlContent)
	if err != nil {
		return fmt.Errorf("errore nell'estrazione delle email: %v", err)
	}

	// Sovrascrivi il CSV con le nuove email
	if err := appendEmailsToCSV(csvFilePath, emails); err != nil {
		return fmt.Errorf("errore nell'aggiornamento del CSV: %v", err)
	}

	fmt.Printf("✅ CSV aggiornato con %d email.\n", len(emails))
	return nil
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
                        🚀 BENVENUTO IN EFFEMMEWEB UTILITY 🚀
======================================================================================================================
███████╗███████╗███████╗███████╗███╗   ███╗███╗   ███╗███████╗██╗    ██╗███████╗██████╗ 
██╔════╝██╔════╝██╔════╝██╔════╝████╗ ████║████╗ ████║██╔════╝██║    ██║██╔════╝██╔══██╗
█████╗  █████╗  █████╗  █████╗  ██╔████╔██║██╔████╔██║█████╗  ██║ █╗ ██║█████╗  ██████╔╝
██╔══╝  ██╔══╝  ██╔══╝  ██╔══╝  ██║╚██╔╝██║██║╚██╔╝██║██╔══╝  ██║███╗██║██╔══╝  ██╔══██╗
███████╗██║     ██║     ███████╗██║ ╚═╝ ██║██║ ╚═╝ ██║███████╗╚███╔███╔╝███████╗██████╔╝
╚══════╝╚═╝     ╚═╝     ╚══════╝╚═╝     ╚═╝╚═╝     ╚═╝╚══════╝ ╚══╝╚══╝ ╚══════╝╚═════╝ 

======================================================================================================================`

    terminalWidth := getTerminalWidth()
    centeredBanner := centerText(banner, terminalWidth)

    title := color.New(color.FgCyan, color.Bold).SprintFunc()
    section := color.New(color.FgGreen, color.Bold).SprintFunc()
    highlight := color.New(color.FgYellow, color.Bold).SprintFunc()
    important := color.New(color.FgRed, color.Bold).SprintFunc()

    // Mostra il banner centrato
    fmt.Println(title(centeredBanner))
    fmt.Println(title("Benvenuto in EFFEMMEWEB Utility, il tuo strumento multifunzione!\n"))

    fmt.Println(section("📘 COME SI USA:"))
    fmt.Println("1️⃣  Assicurati che i seguenti file di configurazione siano nella directory principale:")
    fmt.Printf("   - %s: %s\n", highlight("keyword.csv"), "Parole chiave per lo scraping.")
    fmt.Printf("   - %s: %s\n", highlight("comuni.csv"), "Elenco dei comuni per combinazioni di ricerca.")
    fmt.Printf("   - %s: %s\n", highlight("email_config.json"), "Configurazione email (oggetto e corpo).\n")

    fmt.Println(section("⚙️  MENU PRINCIPALE:"))
    fmt.Printf("   ➤ %s: Avvia lo scraping per raccogliere dati da Google Maps.\n", highlight("1. Scraping"))
    fmt.Println("       📌 Raccoglie dati come Nome Attività, Categoria, Sito Web, Telefono, ecc.")
    fmt.Printf("   ➤ %s: Converte un file CSV esistente in istruzioni SQL.\n", highlight("2. Generazione SQL"))
    fmt.Printf("   ➤ %s: Genera un file email da un CSV per i destinatari.\n", highlight("3. Generazione Email"))
    fmt.Printf("   ➤ %s: Invia email personalizzate utilizzando un file CSV.\n", highlight("4. Invio Email"))
    fmt.Printf("   ➤ %s: Pulisce gli URL nei file CSV per eliminare parametri non necessari.\n", highlight("5. Pulizia URL"))
    fmt.Printf("   ➤ %s: Filtra un CSV in base a categorie specifiche.\n", highlight("6. Filtra CSV"))
    fmt.Printf("   ➤ %s: Invia una email di prova per verificare il sistema di invio email.\n", highlight("7. Email di Test"))
    fmt.Printf("   ➤ %s: Unisce file CSV dalla directory in un unico file.\n", highlight("8. Unisci CSV"))
    fmt.Printf("   ➤ %s: Unisce file VCF dalla directory in un unico file.\n", highlight("9. Unisci VCF"))
    fmt.Printf("   ➤ %s: Esci dall'applicazione.\n", highlight("10. Esci"))

    fmt.Println(section("🔔 NOTE IMPORTANTI:"))
    fmt.Printf("   - %s\n", important("Puoi interrompere l'esecuzione in sicurezza usando CTRL+C."))
    fmt.Println("   - Assicurati di avere una connessione a internet per scraping e invio email.")

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

func getEmailTemplate(config EmailConfig, siteExists bool, protocol string, seoScore float64, cookieBanner, siteStatus, website, technology string) (string, string, error) {
    var templateKey string

    // Determina il templateKey in base ai parametri
    if !siteExists {
        templateKey = "no_website"
    } else if siteStatus == "unavailable" {
        templateKey = "website_unavailable"
    } else if siteStatus == "maintenance" {
        templateKey = "website_under_maintenance"
    } else {
        templateKey = "website_review"
    }

    // Cerca il template nella configurazione
    template, exists := config.Templates[templateKey]
    if !exists {
        return "", "", fmt.Errorf("template non trovato per la chiave: %s", templateKey)
    }

    subject := template.Subject
    body := strings.Join(template.Body, "\n")

    // Sostituzione dinamica dei placeholder
    body = strings.ReplaceAll(body, "{website}", website)
    body = strings.ReplaceAll(body, "{protocol_review}", func() string {
        if protocol == "http" {
            return "Il tuo sito NON utilizza un protocollo sicuro (HTTPS)."
        }
        return "Il tuo sito utilizza un protocollo sicuro (HTTPS)."
    }())
    body = strings.ReplaceAll(body, "{technology_review}", func() string {
        if technology != "" {
            return fmt.Sprintf("Il tuo sito utilizza %s. Possiamo supportarti con questa piattaforma.", technology)
        }
        return "Non abbiamo identificato la tecnologia utilizzata dal sito."
    }())
    body = strings.ReplaceAll(body, "{cookie_review}", func() string {
        if cookieBanner == "present" {
            return "Il sito ha un banner per i cookie."
        }
        return "Il sito NON ha un banner per i cookie."
    }())
    body = strings.ReplaceAll(body, "{performance_review}", func() string {
        if seoScore < 70 {
            return "Le performance del sito sono sotto la media. Consigliamo ottimizzazioni."
        }
        return "Le performance del sito sono buone."
    }())

    return subject, body, nil
}

func mergeCSVFiles() error {
    // Directory dei file CSV
    csvDir := filepath.Join("scraper_results", "csv_results")

    // Trova tutti i file CSV nella directory
    files, err := filepath.Glob(filepath.Join(csvDir, "*.csv"))
    if err != nil || len(files) == 0 {
        return fmt.Errorf("nessun file CSV trovato nella directory %s", csvDir)
    }

    fmt.Println("\n📂 File disponibili:")
    for i, file := range files {
        fmt.Printf("%d. %s\n", i+1, file)
    }

    fmt.Print("\nSeleziona i file CSV da unire, separandoli con una virgola (es. 1,2,3): ")
    var input string
    fmt.Scanln(&input)

    // Parsing dell'input
    selectedIndexes := strings.Split(input, ",")
    selectedFiles := make([]string, 0, len(selectedIndexes))
    for _, indexStr := range selectedIndexes {
        index, err := strconv.Atoi(strings.TrimSpace(indexStr))
        if err != nil || index < 1 || index > len(files) {
            fmt.Printf("⚠️ Indice non valido: %s\n", indexStr)
            continue
        }
        selectedFiles = append(selectedFiles, files[index-1])
    }

    if len(selectedFiles) == 0 {
        return fmt.Errorf("nessun file selezionato")
    }

    // Mappa per tenere traccia dei duplicati
    uniqueEntries := make(map[string]struct{})

    // Nome del file di output
    outputFilePath := filepath.Join(csvDir, "FINISHER_OUTPUT.CSV")

    // Creazione del file di output
    outputFile, err := os.Create(outputFilePath)
    if err != nil {
        return fmt.Errorf("errore nella creazione del file di output: %v", err)
    }
    defer outputFile.Close()

    writer := csv.NewWriter(outputFile)
    defer writer.Flush()

    // Intestazione del CSV
    header := []string{
        "Nome Attività", "Categoria", "Sito Web", "Telefono", "Indirizzo", "Comune", "Provincia", "Email",
        "Protocollo", "Tecnologia", "Cookie Banner", "Hosting Provider", "Mobile Performance",
        "Desktop Performance", "Punteggio SEO", "Disponibilità Sito", "Stato Manutenzione",
    }
    if err := writer.Write(header); err != nil {
        return fmt.Errorf("errore durante la scrittura dell'intestazione: %v", err)
    }

    // Elaborazione dei file selezionati
    for _, filePath := range selectedFiles {
        fmt.Printf("\nUnione del file: %s\n", filePath)

        inputFile, err := os.Open(filePath)
        if err != nil {
            fmt.Printf("❌ Errore nell'apertura del file %s: %v\n", filePath, err)
            continue
        }
        defer inputFile.Close()

        reader := csv.NewReader(inputFile)

        // Salta l'intestazione del file
        _, err = reader.Read()
        if err != nil {
            fmt.Printf("❌ Errore nella lettura dell'intestazione del file %s: %v\n", filePath, err)
            continue
        }

        // Leggi e processa le righe
        for {
            record, err := reader.Read()
            if err == io.EOF {
                break
            }
            if err != nil {
                fmt.Printf("⚠️ Riga saltata a causa di un errore: %v\n", err)
                continue
            }

            if len(record) != len(header) {
                fmt.Printf("⚠️ Riga non valida: %v\n", record)
                continue
            }

            // Chiave univoca basata su Nome Attività, Email e Telefono
            key := strings.ToLower(record[0] + record[7] + record[3])
            if _, exists := uniqueEntries[key]; exists {
                fmt.Printf("🔁 Duplicato trovato: %s\n", record[0])
                continue
            }

            uniqueEntries[key] = struct{}{}

            if err := writer.Write(record); err != nil {
                fmt.Printf("❌ Errore durante la scrittura del record: %v\n", err)
            }
        }
    }

    fmt.Printf("\n✅ File unito creato con successo: %s\n", outputFilePath)
    return nil
}

func mergeVCFFiles() error {
	// Directory dei file VCF
	vcfDir := filepath.Join("scraper_results", "vcf_results")

	// Trova tutti i file VCF nella directory
	files, err := filepath.Glob(filepath.Join(vcfDir, "*.vcf"))
	if err != nil || len(files) == 0 {
		return fmt.Errorf("nessun file VCF trovato nella directory %s", vcfDir)
	}

	fmt.Println("\n📂 File VCF disponibili:")
	for i, file := range files {
		fmt.Printf("%d. %s\n", i+1, file)
	}

	fmt.Print("\nSeleziona i file VCF da unire, separandoli con una virgola (es. 1,2,3): ")
	var input string
	fmt.Scanln(&input)

	// Parsing dell'input
	selectedIndexes := strings.Split(input, ",")
	selectedFiles := make([]string, 0, len(selectedIndexes))
	for _, indexStr := range selectedIndexes {
		index, err := strconv.Atoi(strings.TrimSpace(indexStr))
		if err != nil || index < 1 || index > len(files) {
			fmt.Printf("⚠️ Indice non valido: %s\n", indexStr)
			continue
		}
		selectedFiles = append(selectedFiles, files[index-1])
	}

	if len(selectedFiles) == 0 {
		return fmt.Errorf("nessun file selezionato")
	}

	// Nome del file di output
	outputFilePath := filepath.Join(vcfDir, "merged_output.vcf")

	// Creazione del file di output
	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		return fmt.Errorf("errore nella creazione del file di output: %v", err)
	}
	defer outputFile.Close()

	writer := bufio.NewWriter(outputFile)
	defer writer.Flush()

	// Set per rilevare duplicati
	uniqueContacts := make(map[string]struct{})

	// Unione dei file selezionati
	for _, filePath := range selectedFiles {
		fmt.Printf("\nUnione del file: %s\n", filePath)

		inputFile, err := os.Open(filePath)
		if err != nil {
			fmt.Printf("❌ Errore nell'apertura del file %s: %v\n", filePath, err)
			continue
		}
		defer inputFile.Close()

		scanner := bufio.NewScanner(inputFile)
		var contactBuffer strings.Builder
		isContact := false

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "BEGIN:VCARD") {
				isContact = true
				contactBuffer.Reset()
				contactBuffer.WriteString(line + "\n")
				continue
			}

			if strings.HasPrefix(line, "END:VCARD") {
				isContact = false
				contactBuffer.WriteString(line + "\n")
				contact := contactBuffer.String()

				if _, exists := uniqueContacts[contact]; !exists {
					uniqueContacts[contact] = struct{}{}
					_, err := writer.WriteString(contact)
					if err != nil {
						fmt.Printf("❌ Errore durante la scrittura nel file di output: %v\n", err)
						break
					}
				} else {
					fmt.Println("🔁 Contatto duplicato ignorato.")
				}
				continue
			}

			if isContact {
				contactBuffer.WriteString(line + "\n")
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("❌ Errore durante la lettura del file %s: %v\n", filePath, err)
		}
	}

	fmt.Printf("\n✅ File VCF unito creato con successo: %s\n", outputFilePath)
	return nil
}
func filterCSV(inputFile string, outputFile string) error {
    // Apri il file di input
    file, err := os.Open(inputFile)
    if err != nil {
        return fmt.Errorf("impossibile aprire il file di input: %v", err)
    }
    defer file.Close()

    reader := csv.NewReader(file)
    headers, err := reader.Read() // Leggi l'intestazione
    if err != nil {
        return fmt.Errorf("errore durante la lettura dell'intestazione: %v", err)
    }

    // Trova la colonna "Categoria"
    categoryIndex := -1
    for i, header := range headers {
        if strings.ToLower(strings.TrimSpace(header)) == "categoria" {
            categoryIndex = i
            break
        }
    }

    if categoryIndex == -1 {
        return fmt.Errorf("colonna 'Categoria' non trovata nel file di input")
    }

    // Raccogli tutte le categorie uniche
    categorySet := make(map[string]struct{})
    var rows [][]string

    for {
        record, err := reader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            return fmt.Errorf("errore durante la lettura delle righe: %v", err)
        }

        if len(record) > categoryIndex {
            category := strings.TrimSpace(record[categoryIndex])
            categorySet[category] = struct{}{}
            rows = append(rows, record)
        }
    }

    // Stampa le categorie disponibili
    fmt.Println("📋 Categorie disponibili nel file:")
    categoriesList := make([]string, 0, len(categorySet))
    for category := range categorySet {
        categoriesList = append(categoriesList, category)
    }
    sort.Strings(categoriesList) // Ordina alfabeticamente
    for i, category := range categoriesList {
        fmt.Printf("%d. %s\n", i+1, category)
    }

    // Richiedi le categorie da mantenere
    fmt.Print("\nInserisci i numeri delle categorie da mantenere, separati da virgola: ")
    var input string
    fmt.Scanln(&input)

    selectedIndexes := strings.Split(input, ",")
    selectedCategories := make(map[string]bool)
    for _, indexStr := range selectedIndexes {
        index, err := strconv.Atoi(strings.TrimSpace(indexStr))
        if err != nil || index < 1 || index > len(categoriesList) {
            fmt.Printf("⚠️ Indice non valido: %s\n", indexStr)
            continue
        }
        selectedCategories[categoriesList[index-1]] = true
    }

    // Filtra le righe
    var filteredRows [][]string
    for _, record := range rows {
        if len(record) > categoryIndex && selectedCategories[record[categoryIndex]] {
            filteredRows = append(filteredRows, record)
        }
    }

    // Scrivi il file di output
    outputDir := filepath.Dir(outputFile)
    if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
        return fmt.Errorf("errore nella creazione della directory di output: %v", err)
    }

    output, err := os.Create(outputFile)
    if err != nil {
        return fmt.Errorf("impossibile creare il file di output: %v", err)
    }
    defer output.Close()

    writer := csv.NewWriter(output)
    defer writer.Flush()

    // Scrivi l'intestazione e le righe filtrate
    if err := writer.Write(headers); err != nil {
        return fmt.Errorf("errore durante la scrittura dell'intestazione: %v", err)
    }
    if err := writer.WriteAll(filteredRows); err != nil {
        return fmt.Errorf("errore durante la scrittura delle righe filtrate: %v", err)
    }

    fmt.Printf("✅ File filtrato creato con successo: %s\n", outputFile)
    return nil
}

func selectInputFile(directory string) (string, error) {
    files, err := filepath.Glob(filepath.Join(directory, "*.csv"))
    if err != nil || len(files) == 0 {
        return "", fmt.Errorf("nessun file CSV trovato nella directory %s", directory)
    }

    fmt.Println("📂 File disponibili:")
    for i, file := range files {
        fmt.Printf("%d. %s\n", i+1, file)
    }

    var choice int
    fmt.Print("Seleziona un file CSV (numero): ")
    fmt.Scanln(&choice)

    if choice < 1 || choice > len(files) {
        return "", fmt.Errorf("scelta non valida")
    }

    return files[choice-1], nil
}

func main() {
    var totalRows = new(int)
	clearTerminal()

	printUsage()

    // Aggiorna la lista delle email disiscritte
	if err := updateUnsubscribeList(); err != nil {
		fmt.Println("❌ Errore:", err)
	} else {
		fmt.Println("✅ Operazione completata con successo.")
	}

	// Debug: stampa il contenuto del CSV
	file, err := os.Open("unsubscribe_list_api.csv")
	if err != nil {
		fmt.Println("❌ Errore nella lettura del CSV:", err)
		return
	}
	defer file.Close()

	fmt.Println("Contenuto del CSV aggiornato:")
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

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

            // 🔁 Calcola righe dal CSV interrotto
            if rows, err := cleanAndCountCSVRows(generatedCSV); err == nil {
                *totalRows += rows
            } else {
                fmt.Println("⚠️ Errore nel conteggio righe CSV:", err)
            }            

        } else {
            fmt.Println(color.New(color.FgRed).Sprint("Nessun file CSV generato. Uscita dal programma."))
        }
    
        // 🔴 Usa il valore aggiornato di `totalRows`
        fmt.Printf("Categorie: %v\n", categories)
        fmt.Printf("Totale righe: %d\n", totalRows)
        sendReportEmail(categories, *totalRows)
    
        os.Exit(0)
    }()
    

	// Menu principale
	for {
		fmt.Println("\nSeleziona un'opzione dal menu:")
		fmt.Println("1. 🕵️‍♂️  Avvia lo scraping per raccogliere dati da Google Maps (Richiede connessione Internet).")
		fmt.Println("2. 💾 Converti un file CSV esistente in istruzioni SQL.")
		fmt.Println("3. 📧 Genera un file email da un CSV.")
		fmt.Println("4. 📤 Invia email utilizzando un file CSV.")
		fmt.Println("5. 🧹 Pulisci URL dei siti web.")
		fmt.Println("6. 🔄 Filtra un CSV per categorie specifiche.")
        fmt.Println("7. 📧 Invia una email di prova per verificare il sistema di invio email.")
        fmt.Println("8. 📂 Unisci file CSV dalla directory in un unico file.")
        fmt.Println("9. 📂 Unisci file VCF dalla directory in un unico file.")
        fmt.Println("10. ❌ Esci dall'applicazione.")
		fmt.Print("\n" + color.New(color.FgYellow).Sprint("Scegli un'opzione (1-10): "))

		reader := bufio.NewReader(os.Stdin)
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			if confirmAction("Confermi di voler avviare il processo di scraping? Inserisci 'y' per continuare o 'n' per annullare:") {
                if err := runScrapingFlow(ctx, categories, &generatedCSV, totalRows); err != nil {
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
            fmt.Println("📤 Invia email utilizzando un file CSV.")
            
            emailDir := filepath.Join(baseDir, "email_results")
            csvFile, err := selectInputFile(emailDir) // Cerca i file CSV nella directory delle email
            if err != nil {
                fmt.Println(color.New(color.FgRed).Sprintf("Errore durante la selezione del file CSV: %v", err))
                break
            }
        
            fmt.Printf("Invio email per il file CSV: %s\n", csvFile)
        
            // Configurazione SMTP
            smtpConfig := map[string]string{
                "server":   "mail.effemmeweb.it",
                "port":     "465",
                "user":     "info@effemmeweb.it",
                "password": "Ludovica2021", // Usa la password corretta
            }
        
            // Avvia l'invio delle email utilizzando il file CSV selezionato
            if err := processEmails(ctx, csvFile, "sendmaillog.csv", smtpConfig); err != nil {
                fmt.Println(color.New(color.FgRed).Sprintf("Errore durante l'invio delle email: %v", err))
            } else {
                fmt.Println(color.New(color.FgGreen).Sprint("Email inviate con successo."))
            }        
        
		case "5":
			fmt.Println("Pulizia degli URL nei file CSV in corso...")
			csvDir := filepath.Join(baseDir, "csv_results")
			if err := cleanURLsInCSVFiles(csvDir); err != nil {
				fmt.Println(color.New(color.FgRed).Sprintf("Errore durante la pulizia degli URL nei file CSV: %v", err))
			} else {
				fmt.Println(color.New(color.FgGreen).Sprint("Pulizia degli URL completata con successo."))
			}  
		case "6":
			fmt.Println("🔄 Filtraggio di un file CSV per categorie specifiche in corso...")
            inputDir := filepath.Join(baseDir, "csv_results")
            outputDir := filepath.Join(baseDir, "csv_results/output_filter")

            inputFile, err := selectInputFile(inputDir)
            if err != nil {
                fmt.Println(color.New(color.FgRed).Sprintf("Errore durante la selezione del file di input: %v", err))
                continue
            }

			outputFile := filepath.Join(outputDir, "filtered_output.csv")
            if err := filterCSV(inputFile, outputFile); err != nil {
                fmt.Println(color.New(color.FgRed).Sprintf("Errore durante il filtraggio del CSV: %v", err))
            } else {
                fmt.Println(color.New(color.FgGreen).Sprint("✅ File CSV filtrato generato con successo."))
            }

        case "7":
            fmt.Println(color.New(color.FgCyan).Sprint("\nOpzione selezionata: Manda email di test"))
            fmt.Print("Inserisci l'email del destinatario: ")
            reader := bufio.NewReader(os.Stdin)
            testEmail, _ := reader.ReadString('\n')
            testEmail = strings.TrimSpace(testEmail)
        
            fmt.Print("Il destinatario ha un sito web? (y/n): ")
            hasSite, _ := reader.ReadString('\n')
            hasSite = strings.TrimSpace(strings.ToLower(hasSite))
        
            smtpConfig := map[string]string{
                "server":   "mail.effemmeweb.it",
                "port":     "465",
                "user":     "info@effemmeweb.it",
                "password": "Ludovica2021",
            }
        
            emailConfigPath := "email_config.json" // Percorso del file JSON
            var err error
        
            if hasSite == "y" {
                fmt.Print("Inserisci il dominio del sito (es: www.example.com): ")
                domain, _ := reader.ReadString('\n')
                domain = strings.TrimSpace(domain)
            
                fmt.Print("Inserisci il protocollo del sito (http/https): ")
                protocol, _ := reader.ReadString('\n')
                protocol = strings.TrimSpace(strings.ToLower(protocol))
            
                fmt.Print("Inserisci il SEO Score (numero tra 0 e 100): ")
                seoScoreStr, _ := reader.ReadString('\n')
                seoScoreStr = strings.TrimSpace(seoScoreStr)
                seoScore, _ := strconv.ParseFloat(seoScoreStr, 64)
            
                fmt.Print("Il sito ha un banner per i cookie? (y/n): ")
                cookieBanner, _ := reader.ReadString('\n')
                cookieBanner = strings.TrimSpace(strings.ToLower(cookieBanner))
                cookieBannerValue := "missing"
                if cookieBanner == "y" {
                    cookieBannerValue = "present"
                }
            
                fmt.Print("Inserisci la tecnologia utilizzata (es: WordPress, Shopify, ecc.): ")
                technology, _ := reader.ReadString('\n')
                technology = strings.TrimSpace(technology)
            
                // Invio email utilizzando il template "website_review"
                err = sendCustomEmail(
                    testEmail,
                    "Test",
                    emailConfigPath,
                    smtpConfig["server"],
                    smtpConfig["port"],
                    smtpConfig["user"],
                    smtpConfig["password"],
                    true,
                    protocol,
                    seoScore,
                    cookieBannerValue,
                    "available", // <--- Aggiunto siteStatus
                    domain,
                    technology, // Passa la tecnologia
                )
            } else {
                // Invio email utilizzando il template "no_website"
                err = sendCustomEmail(
                    testEmail,
                    "Test",
                    emailConfigPath,
                    smtpConfig["server"],
                    smtpConfig["port"],
                    smtpConfig["user"],
                    smtpConfig["password"],
                    false,
                    "",    // Nessun protocollo
                    0,     // Nessun punteggio SEO
                    "missing",
                    "no_website",
                    "",
                    "", // Nessuna tecnologia
                )
            }            
        
            if err != nil {
                fmt.Println(color.New(color.FgRed).Sprintf("Errore durante l'invio dell'email di test: %v", err))
            } else {
                fmt.Println(color.New(color.FgGreen).Sprint("Email di test inviata con successo!"))
            }  
        case "8":
            fmt.Println("📂 Unione dei file CSV in corso...")
            if err := mergeCSVFiles(); err != nil {
                fmt.Println(color.New(color.FgRed).Sprintf("Errore durante l'unione dei file CSV: %v", err))
            } else {
                fmt.Println(color.New(color.FgGreen).Sprint("✅ File CSV unito creato con successo!"))
            }   
        case "9":
            fmt.Println("📂 Unione dei file VCF in corso...")
            if err := mergeVCFFiles(); err != nil {
                fmt.Println(color.New(color.FgRed).Sprintf("Errore durante l'unione dei file VCF: %v", err))
            } else {
                fmt.Println(color.New(color.FgGreen).Sprint("✅ File VCF unito creato con successo!"))
            }         
        case "10":
			fmt.Println(color.New(color.FgGreen).Sprint("Uscita dal programma. Arrivederci!"))
			return             
		default:
			fmt.Println(color.New(color.FgRed).Sprint("Opzione non valida. Riprova."))
		}
	}
}

func cleanURLsInCSVFiles(dir string) error {
    files, err := filepath.Glob(filepath.Join(dir, "*.csv"))
    if err != nil {
        return fmt.Errorf("errore nella scansione dei file CSV: %v", err)
    }

    for _, file := range files {
        fmt.Printf("🔄 Pulizia degli URL nel file: %s\n", file)
        if err := cleanURLsInCSV(file); err != nil {
            fmt.Printf("❌ Errore nella pulizia del file %s: %v\n", file, err)
        } else {
            fmt.Printf("✅ File %s pulito con successo.\n", file)
        }
    }

    return nil
}


func cleanURLsInCSV(filePath string) error {
    // Leggi tutto il file in memoria e chiudilo immediatamente
    file, err := os.Open(filePath)
    if err != nil {
        return fmt.Errorf("impossibile aprire il file: %v", err)
    }

    reader := csv.NewReader(file)
    rows, err := reader.ReadAll()
    file.Close() // Chiudi subito il file originale
    if err != nil {
        return fmt.Errorf("errore nella lettura del file CSV: %v", err)
    }

    if len(rows) == 0 {
        return fmt.Errorf("file vuoto")
    }

    // Identifica la colonna "Sito Web" o equivalente
    header := rows[0]
    urlColumnIndex := -1
    for i, col := range header {
        if strings.ToLower(strings.TrimSpace(col)) == "sito web" {
            urlColumnIndex = i
            break
        }
    }

    if urlColumnIndex == -1 {
        return fmt.Errorf("colonna 'Sito Web' non trovata")
    }

    // Pulisce gli URL
    for i, row := range rows {
        if i == 0 || len(row) <= urlColumnIndex {
            continue // Salta l'intestazione o righe non valide
        }

        originalURL := row[urlColumnIndex]
        cleanedURL := cleanURL(originalURL)
        rows[i][urlColumnIndex] = cleanedURL
    }

    // Scrive il file aggiornato in un file temporaneo
    tempFilePath := filePath + ".tmp"
    tempFile, err := os.Create(tempFilePath)
    if err != nil {
        return fmt.Errorf("errore nella creazione del file temporaneo: %v", err)
    }

    writer := csv.NewWriter(tempFile)
    defer writer.Flush()

    if err := writer.WriteAll(rows); err != nil {
        tempFile.Close() // Chiudi il file temporaneo prima di eliminarlo
        os.Remove(tempFilePath) // Rimuovi il file temporaneo in caso di errore
        return fmt.Errorf("errore durante la scrittura del file aggiornato: %v", err)
    }

    // Chiudi il file temporaneo prima di rinominarlo
    if err := tempFile.Close(); err != nil {
        os.Remove(tempFilePath) // Rimuovi il file temporaneo in caso di errore
        return fmt.Errorf("errore durante la chiusura del file temporaneo: %v", err)
    }

    // Rinomina con tentativi multipli in caso di accesso negato
    maxRetries := 5
    for attempt := 1; attempt <= maxRetries; attempt++ {
        err = os.Rename(tempFilePath, filePath)
        if err == nil {
            break // Rinomina riuscita
        }

        if attempt == maxRetries {
            os.Remove(tempFilePath) // Elimina il file temporaneo
            return fmt.Errorf("errore durante il rinominare il file temporaneo: %v", err)
        }

        // Aspetta un po' prima di riprovare
        time.Sleep(500 * time.Millisecond)
    }

    return nil
}

func cleanURL(url string) string {
    if idx := strings.Index(url, "?"); idx != -1 {
        url = url[:idx]
    }
    return strings.TrimSuffix(url, "/")
}

func getExistingCSVFile() string {
	files, err := filepath.Glob("scraper_results/csv_results/*.csv")
	if err != nil || len(files) == 0 {
		fmt.Println(color.New(color.FgRed).Sprint("Nessun file CSV trovato nella directory 'scraper_results/csv_results'."))
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

func runScrapingFlow(ctx context.Context, categories []string, generatedCSV *string, totalRows *int) error {
    fmt.Println("\n📋 Ecco la lista delle categorie disponibili per lo scraping:")
    for i, category := range categories {
        fmt.Printf("   %d. %s\n", i+1, category)
    }

    if !confirmAction("Vuoi procedere con lo scraping per tutte queste categorie?") {
        fmt.Println("Scraping annullato.")
        return nil
    }

    startTime := time.Now()
    fmt.Println("🕵️‍♂️ Avvio dello scraping...")

    resultChannel := make(chan string, len(categories))
    var wg sync.WaitGroup

    maxRetries := 3
    retryDelay := 5 * time.Minute

    for _, category := range categories {
        wg.Add(1)

        go func(category string) {
            defer wg.Done()
            attempt := 1
            var newCSV string
            var err error

            for attempt <= maxRetries {
                newCSV, err = runScrapingForCategory(ctx, category)
                if err == nil {
                    resultChannel <- newCSV
                    return
                }

                fmt.Printf("⚠️ Tentativo %d/%d fallito per la categoria '%s'. Ritento tra %v...\n", attempt, maxRetries, category, retryDelay)
                time.Sleep(retryDelay)
                attempt++
            }

            fmt.Printf("❌ Scraping fallito per la categoria '%s' dopo %d tentativi.\n", category, maxRetries)
        }(category)
    }

    wg.Wait()
    close(resultChannel)

    for newCSV := range resultChannel {
        fmt.Printf("📂 Nuovo file CSV generato: %s\n", newCSV)
        *generatedCSV = newCSV
        rows, err := countCSVRows(newCSV)
        if err == nil {
            *totalRows += rows
        }
    }      

    elapsedTime := time.Since(startTime).Minutes()
    printTimer(int(elapsedTime))

    fmt.Println("🧹 Pulizia automatica degli URL nei CSV generati...")
    if err := cleanURLsInCSVFiles("scraper_results/csv_results"); err != nil {
        fmt.Println("❌ Errore nella pulizia degli URL:", err)
    } else {
        fmt.Println("✅ Pulizia URL completata con successo.")
    }

    fmt.Printf("Categorie: %v\n", categories)
    fmt.Printf("Totale righe: %d\n", totalRows)
    sendReportEmail(categories, *totalRows)
    return nil
}

func sendReportEmail(categories []string, totalRows int) {
    emailReport := fmt.Sprintf(
        "Ciao ragazzi di Effemmeweb,\n\n"+
            "Oggi lo scraper ha elaborato la/e categorie: %s.\n"+
            "Ha registrato un totale di %d contatti nei CSV generati.\n"+
            "Gli URL sono stati già puliti in modo automatico.\n\n"+
            "📊 Report generato automaticamente.",
        strings.Join(categories, ", "), totalRows,
    )

    smtpConfig := map[string]string{
        "server":   "mail.effemmeweb.it",
        "port":     "465",
        "user":     "info@effemmeweb.it",
        "password": "Ludovica2021",
    }

    // Invia email
    sendEmail("info@effemmeweb.it", "📊 Report Scraping Effemmeweb", emailReport, smtpConfig["server"], smtpConfig["port"], smtpConfig["user"], smtpConfig["password"])

    // 🔔 Invia anche su Discord
    discordWebhook := "https://discord.com/api/webhooks/1352620789559590932/HP7kdhBMNjZkWKjMuAWDQ-L6yE51BHixH8i4Ymkj7TrAsYjkepEbUDjlTGwR6vSX142j"
    discordMessage := fmt.Sprintf(
        "📊 **Report Scraping Effemmeweb**\n\n"+
            "🗂️ Categorie elaborate: %s\n"+
            "📬 Contatti trovati: %d\n"+
            "🧹 Gli URL sono stati puliti automaticamente.",
        strings.Join(categories, ", "), totalRows,
    )

    if err := sendDiscordNotification(discordWebhook, discordMessage); err != nil {
        fmt.Printf("⚠️ Errore nell'invio del messaggio Discord: %v\n", err)
    } else {
        fmt.Println("✅ Notifica Discord inviata con successo!")
    }
}

func sendDiscordNotification(webhookURL, message string) error {
    payload := map[string]string{
        "content": message,
    }

    jsonData, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("errore nel creare il payload JSON: %v", err)
    }

    resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("errore nella richiesta POST al webhook Discord: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        body, _ := ioutil.ReadAll(resp.Body)
        return fmt.Errorf("errore Discord webhook (status %d): %s", resp.StatusCode, string(body))
    }

    return nil
}


func countCSVRows(filePath string) (int, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return 0, fmt.Errorf("errore nell'apertura del file CSV: %v", err)
    }
    defer file.Close()

    reader := csv.NewReader(file)
    reader.FieldsPerRecord = -1 // Permette righe con un numero variabile di campi

    count := 0
    for {
        record, err := reader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            return 0, fmt.Errorf("errore nella lettura del file CSV: %v", err)
        }

        // Ignora righe vuote
        if len(record) > 0 && strings.TrimSpace(record[0]) != "" {
            count++
        }
    }

    return count, nil
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
	csvDir := "scraper_results/csv_results"
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

func cleanAndCountCSVRows(filePath string) (int, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return 0, fmt.Errorf("errore nell'apertura del file CSV: %v", err)
    }
    defer file.Close()

    reader := csv.NewReader(file)
    records, err := reader.ReadAll()
    if err != nil {
        return 0, fmt.Errorf("errore nella lettura del file CSV: %v", err)
    }

    if len(records) <= 1 {
        return 0, nil // Se il file ha solo l'intestazione o è vuoto
    }

    headerLen := len(records[0])
    validRows := 0

    for _, record := range records[1:] { // Ignora l'intestazione
        if len(record) == headerLen {
            validRows++
        }
    }

    return validRows, nil
}

func cleanLastRowFromCSV(filePath string) error {
	// Leggi tutte le righe del file in memoria
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("impossibile aprire il file CSV: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("errore nella lettura del file CSV: %v", err)
	}

	if len(rows) > 1 {
		// Controlla se l'ultima riga ha un numero di colonne errato rispetto all'intestazione
		headerLength := len(rows[0])
		lastRow := rows[len(rows)-1]
		if len(lastRow) != headerLength {
			fmt.Println("Ultima riga non valida, rimuovendola...")
			rows = rows[:len(rows)-1]
		}
	} else if len(rows) == 1 && len(rows[0]) == 0 {
		// Rimuovi un file vuoto con una singola riga vuota
		rows = nil
	}

	// Scrivi le righe aggiornate direttamente nel file originale
	outputFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("errore durante l'apertura del file CSV per la sovrascrittura: %v", err)
	}
	defer outputFile.Close()

	writer := csv.NewWriter(outputFile)
	if err := writer.WriteAll(rows); err != nil {
		return fmt.Errorf("errore durante la scrittura nel file CSV: %v", err)
	}
	writer.Flush()

	fmt.Println("File CSV aggiornato con successo.")
	return nil
}

func handleCSVOptions(ctx context.Context, csvFile string) {
    // Recupera la categoria, ad esempio dal nome del file CSV
    category := extractCategoryFromFileName(csvFile) // Estrai la categoria dal nome del file

    for {
        fmt.Printf("\nCosa vuoi fare con il file CSV generato (%s)?\n", csvFile)
        fmt.Println("1. Generare file SQL")
        fmt.Println("2. Generare file email")
        fmt.Println("3. Inviare email")
        fmt.Println("4. Uscire")
        fmt.Print("Scegli un'opzione (1-4): ")

        reader := bufio.NewReader(os.Stdin)
        choice, _ := reader.ReadString('\n')
        choice = strings.TrimSpace(choice)

        select {
        case <-ctx.Done():
            fmt.Println("\nInterruzione rilevata. Uscita dalla funzione.")
            return
        default:
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

    columnIndexes := make(map[string]int)
    requiredColumns := []string{
        "Nome Attività", "Email", "Categoria", "Comune", "Sito Web", "Protocollo",
        "Tecnologia", "Cookie Banner", "Hosting Provider", "Mobile Performance",
        "Desktop Performance", "Punteggio SEO", "Disponibilità Sito", "Stato Manutenzione",
    }

    for i, header := range headers {
        columnIndexes[strings.TrimSpace(header)] = i
    }

    for _, col := range requiredColumns {
        if _, exists := columnIndexes[col]; !exists {
            return fmt.Errorf("colonna mancante nel file CSV: %s", col)
        }
    }

    outputFile, err := os.Create(outputEmailFilePath)
    if err != nil {
        return fmt.Errorf("errore nella creazione del file email CSV: %v", err)
    }
    defer outputFile.Close()

    writer := csv.NewWriter(outputFile)
    defer writer.Flush()

    // Scrivi l'intestazione nel file di output
    if err := writer.Write(requiredColumns); err != nil {
        return fmt.Errorf("errore durante la scrittura dell'intestazione: %v", err)
    }

    for {
        record, err := reader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            return fmt.Errorf("errore durante la lettura del file CSV: %v", err)
        }

        // Estrai l'indice della colonna "Email"
        emailIndex := columnIndexes["Email"]
        email := strings.TrimSpace(record[emailIndex])

        // Verifica che l'email non sia vuota e abbia un formato valido
        if email == "" {
            continue // Salta la riga se l'email è vuota
        }

        if !isValidEmail(email) {
            fmt.Printf("⚠️  Email non valida trovata: %s. Riga ignorata.\n", email)
            continue // Salta la riga se l'email non è valida
        }

        // Prepara la riga da scrivere
        row := make([]string, len(requiredColumns))
        for i, col := range requiredColumns {
            row[i] = record[columnIndexes[col]]
        }

        if err := writer.Write(row); err != nil {
            return fmt.Errorf("errore durante la scrittura del record: %v", err)
        }
    }

    fmt.Printf("File email generato con successo: %s\n", outputEmailFilePath)
    return nil
}

func isValidEmail(email string) bool {
    re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
    return re.MatchString(email)
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

func sendCustomEmail(to, name, emailConfigPath, smtpServer, smtpPort, smtpUser, smtpPassword string, siteExists bool, protocol string, seoScore float64, cookieBanner, siteStatus, website, technology string) error {
    // Carica la configurazione dei template
    config, err := loadEmailConfig(emailConfigPath)
    if err != nil {
        return fmt.Errorf("Errore nel caricamento della configurazione email: %v", err)
    }

    // Ottieni il template corretto basato sui parametri
    subject, body, err := getEmailTemplate(config, siteExists, protocol, seoScore, cookieBanner, siteStatus, website, technology)
    if err != nil {
        return fmt.Errorf("Errore nella selezione del template: %v", err)
    }

    // Sostituisci ulteriori placeholder
    body = strings.ReplaceAll(body, "{name}", name)
    body = strings.ReplaceAll(body, "{email}", to) // Aggiunta sostituzione di {email}

    // Invia l'email utilizzando le configurazioni SMTP
    if err := sendEmail(to, subject, body, smtpServer, smtpPort, smtpUser, smtpPassword); err != nil {
        return fmt.Errorf("Errore durante l'invio dell'email a %s: %v", to, err)
    }

    return nil
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

    emailConfig, err := loadEmailConfig(emailConfigPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Errore durante il caricamento della configurazione email: %v\n", err)
        return
    }

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

            // Dati della riga
            name := emailData[0]
            email := emailData[1]
            website := emailData[4]
            protocol := emailData[5]
            technology := emailData[6]
            cookieBanner := emailData[7]
            mobilePerf, _ := strconv.ParseFloat(emailData[9], 64)
            desktopPerf, _ := strconv.ParseFloat(emailData[10], 64)
            siteAvailable := emailData[12]
            siteMaintenance := emailData[13]

            // Escludi le email che contengono "@pec."
            if strings.Contains(strings.ToLower(email), "@pec.") {
                fmt.Printf("⚠️  Email con '@pec.' trovata: %s. Riga ignorata.\n", email)
                continue
            }

            if _, exists := log[email]; exists {
                fmt.Printf("Email già inviata: %s\n", email)
                continue
            }

            var subject, body string

            if website == "" {
                subject = emailConfig.Templates["no_website"].Subject
                body = strings.Join(emailConfig.Templates["no_website"].Body, "\n")
            } else if siteAvailable == "Non Disponibile" {
                subject = emailConfig.Templates["website_unavailable"].Subject
                body = strings.ReplaceAll(strings.Join(emailConfig.Templates["website_unavailable"].Body, "\n"), "{website}", website)
            } else if siteMaintenance == "In Manutenzione" {
                subject = emailConfig.Templates["website_under_maintenance"].Subject
                body = strings.ReplaceAll(strings.Join(emailConfig.Templates["website_under_maintenance"].Body, "\n"), "{website}", website)
            } else {
                subject = emailConfig.Templates["website_review"].Subject

                // Gestione del Protocollo
                var protocolReview string
                if protocol == "http" {
                    protocolReview = "Il tuo sito non ha un certificato SSL attivo."
                } else if protocol == "https" {
                    protocolReview = "Il tuo sito utilizza un protocollo sicuro (HTTPS)."
                } else {
                    protocolReview = "Il tuo sito utilizza un protocollo sconosciuto."
                }

                // Gestione della Tecnologia
                var technologyReview string
                switch technology {
                case "WordPress", "Shopify", "Prestashop", "Magento":
                    technologyReview = fmt.Sprintf("Il tuo sito utilizza %s. Possiamo supportarti con questa piattaforma.", technology)
                case "":
                    technologyReview = "La tecnologia utilizzata dal tuo sito non è stata identificata."
                default:
                    technologyReview = fmt.Sprintf("Il tuo sito utilizza %s.", technology)
                }

                // Gestione del Cookie Banner
                var cookieReview string
                if cookieBanner == "Sì" {
                    cookieReview = "Abbiamo trovato il banner per i cookie."
                } else if cookieBanner == "No" {
                    cookieReview = "Non abbiamo trovato un banner per i cookie. Ti consigliamo di aggiungerlo per rispettare le normative sulla privacy."
                } else {
                    cookieReview = "La presenza del banner per i cookie non è stata determinata."
                }

                // Gestione delle Performance
                var perfReview string
                avgPerf := (mobilePerf + desktopPerf) / 2
                if avgPerf < 70 {
                    perfReview = "La media delle performance è inferiore a 70. Consigliamo un'ottimizzazione."
                } else {
                    perfReview = "Le performance del tuo sito sono buone."
                }

                // Sostituzione dei placeholder nel corpo dell'email
                body = strings.ReplaceAll(strings.Join(emailConfig.Templates["website_review"].Body, "\n"), "{website}", website)
                body = strings.ReplaceAll(body, "{protocol_review}", protocolReview)
                body = strings.ReplaceAll(body, "{technology_review}", technologyReview)
                body = strings.ReplaceAll(body, "{cookie_review}", cookieReview)
                body = strings.ReplaceAll(body, "{performance_review}", perfReview)
            }

            // Sostituisci {name} e {email} nei placeholder
            body = strings.ReplaceAll(body, "{name}", name)
            body = strings.ReplaceAll(body, "{email}", email) // Aggiunta sostituzione di {email}

            // Invia l'email
            err := sendEmail(email, subject, body, smtpConfig["server"], smtpConfig["port"], smtpConfig["user"], smtpConfig["password"])
            if err != nil {
                fmt.Fprintf(os.Stderr, "Errore durante l'invio dell'email a %s: %v\n", email, err)
                updateSendMailLog(logPath, name, email, "Errore", subject)
            } else {
                fmt.Printf("Email inviata a %s\n", email)
                updateSendMailLog(logPath, name, email, "Inviata", subject)
            }
        }
    }
}

func readUnsubscribedEmails(filePath string) (map[string]struct{}, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return nil, fmt.Errorf("errore durante l'apertura del file di disiscrizione: %v", err)
    }
    defer file.Close()

    content, err := ioutil.ReadAll(file)
    if err != nil {
        return nil, fmt.Errorf("errore durante la lettura del file di disiscrizione: %v", err)
    }

    emails := strings.Fields(string(content)) // Divide il contenuto in base agli spazi
    unsubscribed := make(map[string]struct{}, len(emails))
    for _, email := range emails {
        unsubscribed[strings.TrimSpace(email)] = struct{}{}
    }

    return unsubscribed, nil
}

func processEmails(ctx context.Context, emailCSV, logPath string, smtpConfig map[string]string) error {
    startTime := time.Now()  // Inizia a tracciare il tempo
    fmt.Println("Inizio processo di invio email...")

    // Leggi le email di disiscrizione
    unsubscribedEmails, err := readUnsubscribedEmails("unsubscribe_list_api.csv")
    if err != nil {
        return fmt.Errorf("errore durante la lettura delle email disiscritte: %v", err)
    }

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

    var allEmails [][]string
    for {
        record, err := reader.Read()
        if err == io.EOF {
            break
        }
        if err != nil || len(record) < 2 {
            continue
        }

        email := strings.TrimSpace(record[1])
        if _, unsubscribed := unsubscribedEmails[email]; unsubscribed {
            fmt.Printf("⚠️  Email nella lista di disiscrizione: %s. Riga ignorata.\n", email)
            continue
        }

        allEmails = append(allEmails, record)
    }

    // Filtra le email con "@pec."
    var filteredEmails [][]string
    for _, record := range allEmails {
        email := strings.TrimSpace(record[1])
        if strings.Contains(strings.ToLower(email), "@pec.") {
            fmt.Printf("⚠️  Email con '@pec.' trovata: %s. Riga ignorata.\n", email)
            continue
        }
        filteredEmails = append(filteredEmails, record)
    }

    // Imposta il contatore per il numero massimo di email giornaliere
    emailCount := 0
    maxEmailsPerDay := 600

    for _, emailData := range filteredEmails {
        select {
        case <-ctx.Done():
            fmt.Println("Processo interrotto.")
            return nil
        default:
        }

        // Controlla se è fuori dall'orario consentito (9-18 orario italiano)
        now := time.Now().In(time.FixedZone("CET", 1*60*60)) // Orario italiano
        if now.Hour() < 9 || now.Hour() >= 18 {
            fmt.Println("⏳ Fuori dall'orario di invio (9-18). Aspetto fino al prossimo orario consentito...")
            for {
                time.Sleep(10 * time.Minute) // Aspetta 10 minuti
                now = time.Now().In(time.FixedZone("CET", 1*60*60))
                if now.Hour() >= 9 && now.Hour() < 18 {
                    break
                }
            }
        }

        // Verifica se si è raggiunto il limite massimo giornaliero
        if emailCount >= maxEmailsPerDay {
            fmt.Println("⏳ Raggiunto il limite massimo giornaliero di email (600). Termino il processo per oggi.")
            break
        }

        // Invio dell'email
        wg := &sync.WaitGroup{}
        emailQueue := make(chan []string, 1)
        emailQueue <- emailData
        close(emailQueue)
        wg.Add(1)
        go sendEmailsFromQueue(ctx, emailQueue, smtpConfig, logPath, "email_config.json", wg)
        wg.Wait()

        emailCount++
        fmt.Printf("✅ Email %d inviata con successo.\n", emailCount)

        // Attendi un ritardo casuale tra 30 secondi e 1 minuto
        delay := time.Duration(30+rand.New(rand.NewSource(time.Now().UnixNano())).Intn(31)) * time.Second
        fmt.Printf("⏳ Attesa di %s prima del prossimo invio...\n", delay)
        select {
        case <-ctx.Done():
            fmt.Println("Processo interrotto durante l'attesa.")
            return nil
        case <-time.After(delay):
        }

        // Dopo 100 email, pausa per 60 minuti
        if emailCount%100 == 0 {
            fmt.Println("⏳ Pausa di 60 minuti dopo 100 email inviate...")
            select {
            case <-ctx.Done():
                fmt.Println("Processo interrotto durante la pausa.")
                return nil
            case <-time.After(60 * time.Minute):
            }
        }
    }

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

