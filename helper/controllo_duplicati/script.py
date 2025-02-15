import pandas as pd

# Specifica il nome del file CSV di input
input_file = 'final_output.csv'
# Specifica il nome del file CSV di output
output_file = 'output.csv'

# Leggi il file CSV in un DataFrame
df = pd.read_csv(input_file)

# Rimuovi le righe duplicate
df_unique = df.drop_duplicates()

# Salva il DataFrame risultante in un nuovo file CSV (senza l'indice)
df_unique.to_csv(output_file, index=False)

print(f"File {input_file} elaborato. Duplicati rimossi e salvati in {output_file}.")
