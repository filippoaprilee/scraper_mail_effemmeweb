import pandas as pd

# Caricare il file CSV
file_path = "input.csv"  # Modifica con il percorso corretto

# Leggere il file CSV
df = pd.read_csv(file_path)

# Trovare i duplicati basati su effemmeweb_telefono1 ed effemmeweb_nome
duplicati = df[df.duplicated(subset=["EFFEMMEWEB_TELEFONO1", "EFFEMMEWEB_NOME"], keep=False)]

# Filtrare solo quelli con EFFEMMEWEB_FLAG_CHIAMATO e EFFEMMEWEB_FLAG_RISPOSTO uguale a 0
duplicati_filtrati = duplicati[
    (duplicati["EFFEMMEWEB_FLAG_CHIAMATO"] == 0) & (duplicati["EFFEMMEWEB_FLAG_RISPOSTO"] == 0)
]

# Salvare i duplicati filtrati in un nuovo file
duplicati_filtrati.to_csv("duplicati_filtrati.csv", index=False)

# Stampare i risultati
print("Duplicati trovati con FLAG_CHIAMATO e FLAG_RISPOSTO = 0:", len(duplicati_filtrati))
print(duplicati_filtrati.head())
