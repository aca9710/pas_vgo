#!/usr/bin/env python3
"""
Convertidor de Markdown a PDF usando pandoc.
Uso: python conv.py [archivo.md] [-o pdf]
"""
import subprocess
import sys
import os

def convert_md_to_pdf(md_file, pdf_file=None, css_file=None):
    if pdf_file is None:
        pdf_file = md_file.replace('.md', '.pdf')
    
    if not os.path.exists(md_file):
        print(f"Error: No se encontró el archivo {md_file}", file=sys.stderr)
        return False
    
    cmd = [
        'pandoc',
        md_file,
        '-o', pdf_file,
        '--pdf-engine=xelatex',
        '-V', 'mainfont=DejaVuSans',
        '-V', 'geometry:margin=1in',
        '--wrap=none'
    ]
    
    if css_file and os.path.exists(css_file):
        cmd.insert(2, '-c')
        cmd.insert(3, css_file)
    
    try:
        result = subprocess.run(cmd, capture_output=True, text=True)
        if result.returncode == 0:
            print(f"PDF generado: {pdf_file}")
            return True
        else:
            print(f"Error: {result.stderr}", file=sys.stderr)
            return False
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        return False

if __name__ == '__main__':
    md_file = 'endpoint_ref.md'
    pdf_file = None
    
    for arg in sys.argv[1:]:
        if arg.endswith('.md'):
            md_file = arg
        elif arg == '-o' and len(sys.argv) > sys.argv.index(arg) + 1:
            pdf_file = sys.argv[sys.argv.index(arg) + 1]
        elif not arg.startswith('-'):
            if arg.endswith('.pdf'):
                pdf_file = arg
            else:
                md_file = arg
    
    if not pdf_file:
        pdf_file = md_file.replace('.md', '.pdf')
    
    convert_md_to_pdf(md_file, pdf_file)