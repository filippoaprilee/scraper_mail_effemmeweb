import csv

def load_comuni_mapping(mapping_file):
    """
    Legge il file mapping dei comuni e restituisce un dizionario:
    { nome_comune_normalizzato: ID }
    """
    mapping = {}
    with open(mapping_file, newline='', encoding='utf-8') as f:
        reader = csv.DictReader(f)
        for row in reader:
            comune = row['COMUNE'].strip().upper()  # normalizza il nome
            mapping[comune] = row['ID'].strip()
    return mapping

def convert_output(mapping_file, input_file, output_file):
    # Carica il mapping dei comuni (nome -> ID)
    comuni_mapping = load_comuni_mapping(mapping_file)
    
    # Legge il file output.csv
    with open(input_file, newline='', encoding='utf-8') as f_in:
        reader = csv.DictReader(f_in)
        # Conserva l'ordine dei campi come presente nel file
        fieldnames = reader.fieldnames
        rows = list(reader)
    
    # Sostituisce il campo "Comune" in ogni riga con l'ID corrispondente
    for row in rows:
        comune_originale = row['Comune'].strip().upper()
        # Se il comune è presente nel mapping, sostituisce con l'ID, altrimenti lascia vuoto
        row['Comune'] = comuni_mapping.get(comune_originale, "")
    
    # Scrive il risultato nel nuovo file CSV
    with open(output_file, 'w', newline='', encoding='utf-8') as f_out:
        writer = csv.DictWriter(f_out, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(rows)

if __name__ == "__main__":
    mapping_file = "comuni-_1_.csv"   # file di mapping dei comuni
    input_file = "output.csv"          # file di input con tutti i dati
    output_file = "final_output.csv"   # file finale con il comune sostituito con l'ID
    
    convert_output(mapping_file, input_file, output_file)
