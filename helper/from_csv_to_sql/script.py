import csv

# Nome del file CSV di input e del file SQL di output
input_file = 'input.csv'
output_file = 'output.sql'

def escape_sql(value):
    """
    Escapa gli apici singoli per essere utilizzati in una stringa SQL.
    Raddoppia ogni apice singolo.
    """
    return value.replace("'", "''")

# Apre il file CSV in modalità lettura e il file SQL in modalità scrittura
with open(input_file, mode='r', newline='', encoding='utf-8') as infile, \
     open(output_file, mode='w', encoding='utf-8') as outfile:
    
    # Scrive l'inizio del blocco SQL
    outfile.write("BEGIN\n")
    
    # Crea il reader CSV (con delimitatore `;`)
    reader = csv.reader(infile, delimiter=';')
    
    # Salta l'intestazione
    headers = next(reader)
    
    # Per ogni riga del CSV, genera l'istruzione SQL
    for row in reader:
        # Controlla che la riga abbia tutti i valori richiesti
        if len(row) < 6:
            print(f"Riga incompleta ignorata: {row}")
            continue
        
        # Estraggo i campi dalla riga
        id, fk_progetto_id, link, username, password, dettagli = row
        
        # Escapa i valori SQL
        id = escape_sql(id)
        fk_progetto_id = escape_sql(fk_progetto_id)
        link = escape_sql(link)
        username = escape_sql(username)
        password = escape_sql(password)
        dettagli = escape_sql(dettagli)
        
        # Genera l'istruzione SQL con apici singoli raddoppiati
        insert_stmt = (
            f"INSERT INTO DETTAGLI_PROGETTI (ID, FK_PROGETTO_ID, LINK, USERNAME, PASSWORD, DETTAGLI) "
            f"VALUES (''{id}'', ''{fk_progetto_id}'', ''{link}'', ''{username}'', ''{password}'', ''{dettagli}'')"
        )
        
        # Scrive il comando nel file
        outfile.write(f"EXECUTE IMMEDIATE '{insert_stmt}';\n")
    
    # Chiude il blocco SQL
    outfile.write("END;\n")
