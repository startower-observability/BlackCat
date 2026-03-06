---
name: Document Processing
tags: [pdf, docx, xlsx, pptx, documents, extraction]
requires:
  bins: [python3]
---

# Document Processing

Extract, convert, and analyze content from PDF, DOCX, PPTX, and XLSX files.

## Prerequisites

- `python3` binary
- Required packages (install as needed):
  - `pip install pypdf2 pdfplumber python-docx openpyxl python-pptx`

## Capabilities

- Extract text from PDFs with layout preservation
- Read and edit DOCX files
- Parse XLSX spreadsheets
- Extract text and shapes from PPTX
- Convert between formats
- Summarize large documents

## How to Use

### Extract text from PDF

```bash
pip install pdfplumber
```

```python
import pdfplumber

with pdfplumber.open("document.pdf") as pdf:
    full_text = ""
    for page in pdf.pages:
        full_text += page.extract_text() + "\n\n"
    
print(full_text[:2000])  # First 2000 characters
```

### Extract tables from PDF

```python
import pdfplumber

with pdfplumber.open("report.pdf") as pdf:
    for i, page in enumerate(pdf.pages):
        tables = page.extract_tables()
        for table in tables:
            for row in table:
                print(row)
```

### Read DOCX file

```bash
pip install python-docx
```

```python
from docx import Document

doc = Document("document.docx")

# Extract all paragraphs
for para in doc.paragraphs:
    print(para.text)

# Extract tables
for table in doc.tables:
    for row in table.rows:
        print([cell.text for cell in row.cells])
```

### Read XLSX spreadsheet

```bash
pip install openpyxl
```

```python
from openpyxl import load_workbook

wb = load_workbook("data.xlsx")
sheet = wb.active

# Read all rows
for row in sheet.iter_rows(values_only=True):
    print(row)

# Read specific range
for row in sheet["A1:D10"]:
    print([cell.value for cell in row])
```

### Read PPTX presentation

```bash
pip install python-pptx
```

```python
from pptx import Presentation

prs = Presentation("slides.pptx")

for slide_num, slide in enumerate(prs.slides, 1):
    print(f"\n=== Slide {slide_num} ===")
    for shape in slide.shapes:
        if hasattr(shape, "text"):
            print(shape.text)
```

### Summarize large PDF

```python
import pdfplumber

def extract_and_summarize(pdf_path):
    with pdfplumber.open(pdf_path) as pdf:
        text = ""
        for page in pdf.pages[:10]:  # First 10 pages
            text += page.extract_text() + "\n"
    
    # Send to LLM for summary
    summary_prompt = f"Summarize this document:\n\n{text[:8000]}"
    return summary_prompt

print(extract_and_summarize("large_doc.pdf"))
```

## Format Conversion

### PDF to text file

```python
import pdfplumber

with pdfplumber.open("input.pdf") as pdf:
    with open("output.txt", "w", encoding="utf-8") as out:
        for page in pdf.pages:
            out.write(page.extract_text() + "\n\n---PAGE BREAK---\n\n")
```

### DOCX to plain text

```python
from docx import Document

doc = Document("input.docx")
with open("output.txt", "w", encoding="utf-8") as out:
    for para in doc.paragraphs:
        out.write(para.text + "\n")
```

## Notes

- `pdfplumber` preserves layout better than `pypdf2` for complex PDFs
- Scanned PDFs require OCR (use `pytesseract` + `pdf2image`)
- Large files: process page by page to avoid memory issues
- XLSX formulas: use `cell.value` for computed values
- DOCX styles: access via `para.style.name`
