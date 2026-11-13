import { PDFDocument, StandardFonts, rgb } from 'pdf-lib'

const CYRILLIC: Record<string, string> = {
  а: 'a', б: 'b', в: 'v', г: 'g', д: 'd', е: 'e', ё: 'e', ж: 'zh', з: 'z',
  и: 'i', й: 'y', к: 'k', л: 'l', м: 'm', н: 'n', о: 'o', п: 'p', р: 'r',
  с: 's', т: 't', у: 'u', ф: 'f', х: 'h', ц: 'ts', ч: 'ch', ш: 'sh', щ: 'sch',
  ъ: '', ы: 'y', ь: '', э: 'e', ю: 'yu', я: 'ya',
}

/** Fixture PDFs use a standard font, so Cyrillic is transliterated. The API returns the real file. */
export function toWinAnsi(value: string): string {
  const latin = Array.from(value)
    .map((char) => {
      const mapped = CYRILLIC[char.toLowerCase()]
      if (mapped === undefined) return char
      if (char.toLowerCase() === char) return mapped
      return mapped.charAt(0).toUpperCase() + mapped.slice(1)
    })
    .join('')
  return latin.replace(/[^\t\n\r\x20-\x7E]/g, '?')
}

export async function buildPdf(title: string, headers: string[], rows: string[][]): Promise<Uint8Array> {
  const doc = await PDFDocument.create()
  const font = await doc.embedFont(StandardFonts.Helvetica)
  const bold = await doc.embedFont(StandardFonts.HelveticaBold)
  let page = doc.addPage([842, 595])
  let y = 560
  const draw = (text: string, size: number, used = font) => {
    if (y < 40) {
      page = doc.addPage([842, 595])
      y = 560
    }
    page.drawText(toWinAnsi(text).slice(0, 180), {
      x: 32,
      y,
      size,
      font: used,
      color: rgb(12 / 255, 16 / 255, 19 / 255),
    })
    y -= size + 4
  }
  draw(title, 16, bold)
  y -= 6
  draw(headers.join('  '), 8, bold)
  for (const row of rows) draw(row.join('  '), 8)
  if (rows.length === 0) draw('—', 8)
  return doc.save()
}

function xml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

export function buildXls(headers: string[], rows: string[][]): string {
  const header = headers
    .map((cell) => `<Cell><Data ss:Type="String">${xml(cell)}</Data></Cell>`)
    .join('')
  const body = rows
    .map((row) => {
      const cells = row.map((cell) => `<Cell><Data ss:Type="String">${xml(cell)}</Data></Cell>`).join('')
      return `<Row>${cells}</Row>`
    })
    .join('')
  return `<?xml version="1.0" encoding="UTF-8"?>
<?mso-application progid="Excel.Sheet"?>
<Workbook xmlns="urn:schemas-microsoft-com:office:spreadsheet" xmlns:ss="urn:schemas-microsoft-com:office:spreadsheet">
<Worksheet ss:Name="Leave"><Table><Row>${header}</Row>${body}</Table></Worksheet>
</Workbook>`
}
