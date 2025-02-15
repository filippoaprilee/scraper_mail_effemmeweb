import pandas as pd

# Specifica i nomi dei file CSV
file1 = 'file1.csv'  # Contiene: COGNOME, NOME
file2 = 'file2.csv'  # Contiene: Codice fiscale, Codice alternativo 1, Indirizzo, Comune, Provincia, Cap

# Leggi i due file CSV
df1 = pd.read_csv(file1)
df2 = pd.read_csv(file2)

# Verifica che il numero di righe sia uguale
if len(df1) != len(df2):
    raise ValueError("I file CSV hanno un numero di righe diverso. Non è possibile unire i file basandosi sull'ordine delle righe.")

# Unisci i due DataFrame orizzontalmente (concatenazione lungo le colonne)
df3 = pd.concat([df1, df2], axis=1)

# Riordina le colonne secondo l'ordine richiesto
colonne_ordinate = ['COGNOME', 'NOME', 'Codice fiscale', 'Codice alternativo 1', 'Indirizzo', 'Comune', 'Provincia', 'Cap']
df3 = df3[colonne_ordinate]

# Salva il risultato in un nuovo file CSV
df3.to_csv('file3.csv', index=False)

print("File 'file3.csv' creato con successo!")
