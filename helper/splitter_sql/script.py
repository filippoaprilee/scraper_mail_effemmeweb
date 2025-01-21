import os
import re

def split_sql_file(input_file, output_dir, lines_per_file=1000):
    # Leggi il contenuto del file
    with open(input_file, 'r', encoding='utf-8') as file:
        content = file.read()

    # Estrai solo le righe con EXECUTE IMMEDIATE
    matches = re.findall(r"EXECUTE IMMEDIATE '.+?';", content, re.DOTALL)

    # Conta le righe trovate
    total_lines = len(matches)
    print(f"Totale righe trovate: {total_lines}")

    # Dividi in blocchi di massimo lines_per_file righe
    os.makedirs(output_dir, exist_ok=True)
    for i in range(0, total_lines, lines_per_file):
        chunk = matches[i:i + lines_per_file]

        # Crea il contenuto del file
        file_content = "BEGIN\n    " + "\n    ".join(chunk) + "\nEND\n"

        # Nome del file
        output_file = os.path.join(output_dir, f"output_part_{i // lines_per_file + 1}.sql")

        # Scrivi il file
        with open(output_file, 'w', encoding='utf-8') as out_file:
            out_file.write(file_content)

        print(f"Creato file: {output_file}")

# Parametri
input_file = "input.sql"  # File di input
output_dir = "output_files"  # Directory per i file di output
lines_per_file = 1500  # Numero massimo di righe per file

# Esegui lo script
split_sql_file(input_file, output_dir, lines_per_file)