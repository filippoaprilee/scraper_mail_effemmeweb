import cx_Oracle

# Connessione al database
dsn_tns = cx_Oracle.makedsn("HOST", "PORT", service_name="SERVICE_NAME")
conn = cx_Oracle.connect(user="USERNAME", password="PASSWORD", dsn=dsn_tns)
cursor = conn.cursor()

# Recuperare il file
query = "SELECT file_content FROM my_export_files WHERE file_name = 'schema_export_2.dmp'"
cursor.execute(query)
row = cursor.fetchone()

# Scrivere il file su disco
file_name = "schema_export_2.dmp"
blob_data = row[0].read()

with open(file_name, "wb") as file:
    file.write(blob_data)

print(f"✅ File {file_name} scaricato con successo!")

# Chiudere connessione
cursor.close()
conn.close()
