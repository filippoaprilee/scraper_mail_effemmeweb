import csv

# Definiamo i prefissi che indicano un possibile doppio cognome
SURNAME_PREFIXES = {"DA", "DE", "DEL", "DI", "D'", "DEI", "LA", "LE", "LO", "VAN", "VON"}

def process_row(row):
    """
    Processa una riga del CSV e restituisce una tupla (cognome, nome)
    seguendo le regole:
      - Se il primo campo (ripulito) inizia con "A.S.D." → tutto in NOME.
      - Se la riga ha due colonne e il primo campo (ripulito) è un prefisso
        in SURNAME_PREFIXES, allora il cognome è: [prefisso] + " " + [prima parola del secondo campo],
        e il resto (eventualmente vuoto) va in NOME.
      - Altrimenti, se ci sono 2 colonne, il primo campo è il cognome e il secondo il nome.
      - Se c’è una sola colonna, si procede come nel caso classico:
           * se inizia per "A.S.D." → tutto in NOME,
           * se il primo token è un prefisso → unisci il primo e il secondo token come cognome,
           * altrimenti il primo token è il cognome e il resto il nome.
    """
    # Rimuoviamo eventuali spazi superflui e semicolon iniziali
    if not row:
        return "", ""
    
    # Se la riga ha due colonne
    if len(row) >= 2:
        field1 = row[0].lstrip(';').strip()
        field2 = row[1].strip()
        
        # Caso ragione sociale: se field1 inizia con "A.S.D." (ignora case)
        if field1.upper().startswith("A.S.D."):
            # Se field2 è presente, combiniamo i due campi con uno spazio
            full_name = f"{field1} {field2}" if field2 else field1
            return "", full_name
        
        # Se il primo campo è un prefisso noto, assumiamo doppio cognome:
        if field1.upper() in SURNAME_PREFIXES:
            tokens = field2.split()
            if tokens:
                # Il cognome è il prefisso (field1) + la prima parola del secondo campo
                surname = f"{field1} {tokens[0]}"
                name = " ".join(tokens[1:]) if len(tokens) > 1 else ""
                return surname, name
            else:
                return field1, field2
        else:
            # Altrimenti, usiamo field1 come cognome e field2 come nome
            return field1, field2

    # Se la riga ha una sola colonna, la processiamo come stringa intera
    elif len(row) == 1:
        full_name = row[0].lstrip(';').strip()
        if full_name.upper().startswith("A.S.D."):
            return "", full_name
        tokens = full_name.split()
        if len(tokens) > 1:
            if tokens[0].upper() in SURNAME_PREFIXES:
                surname = f"{tokens[0]} {tokens[1]}"
                name = " ".join(tokens[2:]) if len(tokens) > 2 else ""
                return surname, name
            else:
                return tokens[0], " ".join(tokens[1:])
        else:
            return full_name, ""

def main():
    input_file = "prova2.csv"    # File di input
    output_file = "output.csv"  # File di output

    # Legge il file di input (si assume che il separatore sia la virgola)
    with open(input_file, newline='', encoding='utf-8') as csv_in:
        reader = csv.reader(csv_in, delimiter=',')
        rows = list(reader)
    
    # Scrive il file di output usando la virgola come separatore (senza semicolon extra)
    with open(output_file, 'w', newline='', encoding='utf-8') as csv_out:
        writer = csv.writer(csv_out, delimiter=',')
        writer.writerow(["COGNOME", "NOME"])
        
        for row in rows:
            # Se la riga è vuota o contiene solo spazi, la saltiamo

            surname, name = process_row(row)
            writer.writerow([surname, name])

if __name__ == "__main__":
    main()
