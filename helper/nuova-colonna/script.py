import pandas as pd

# Funzione per determinare il sesso in base al nome (euristica semplice)
def determina_sesso(nome):
    # Convertiamo il nome in minuscolo per evitare problemi di confronto
    if nome.strip().lower().endswith('a'):
        return 'F'
    else:
        return 'M'

# Nome del file CSV di input e output
input_file = 'input.csv'
output_file = 'output_con_sesso.csv'

# Leggi il file CSV in un DataFrame
df = pd.read_csv(input_file)

# Aggiungi la nuova colonna "SESSO" basandoti sul campo "NOME"
df['SESSO'] = df['NOME'].apply(determina_sesso)

# Salva il DataFrame risultante in un nuovo file CSV (senza indice)
df.to_csv(output_file, index=False)

print(f"File {input_file} elaborato. Nuovo file con colonna 'SESSO' salvato in {output_file}.")
