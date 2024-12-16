import pandas as pd
import subprocess
import re
import socket
import whois

def esegui_nslookup(sito):
    try:
        # Esegui il comando nslookup e cattura l'output
        result = subprocess.run(["nslookup", sito], capture_output=True, text=True)
        output = result.stdout
        # Estrai l'IP o il nameserver dall'output
        match = re.search(r'Name:\s*(\S+).*?Address:\s*(\S+)', output, re.DOTALL)
        if match:
            return match.group(2)
        else:
            return "N/A"
    except Exception as e:
        return "Errore"

def esegui_whois(sito):
    try:
        # Estraggo il dominio pulito
        dominio = sito.split('/')[0]
        whois_result = whois.whois(dominio)
        if whois_result.registrar:
            return whois_result.registrar
        return "N/A"
    except Exception:
        return "Errore"

def filtra_sconosciuto(input_csv, output_csv):
    # Legge il file CSV
    df = pd.read_csv(input_csv)

    # Seleziona solo le colonne richieste
    colonne_ridotte = ["Nome Attività", "Sito Web"]
    righe_sconosciuto = df[colonne_ridotte]

    # Colonne per il risultato del nslookup e whois
    nslookup_results = []
    whois_results = []
    
    for _, row in righe_sconosciuto.iterrows():
        sito = row.get("Sito Web", "")
        if pd.notna(sito) and sito.strip():
            dominio = sito.replace("https://", "").replace("http://", "").split('/')[0]
            nslookup_results.append(esegui_nslookup(dominio))
            whois_results.append(esegui_whois(dominio))
        else:
            nslookup_results.append("N/A")
            whois_results.append("N/A")

    # Aggiunge il risultato del nslookup e provider al dataframe
    righe_sconosciuto["NSLOOKUP"] = nslookup_results
    righe_sconosciuto["PROVIDER"] = whois_results

    # Salva solo le colonne desiderate
    righe_sconosciuto = righe_sconosciuto[["Nome Attività", "Sito Web", "NSLOOKUP", "PROVIDER"]]
    righe_sconosciuto.to_csv(output_csv, index=False)

    print(f"File salvato come: {output_csv}")

# Esegui la funzione con il nome del file di input e di output
input_file = "input.csv"  # Sostituisci con il nome del tuo file
output_file = "output_sconosciuto.csv"  # Nome del file di output
filtra_sconosciuto(input_file, output_file)