using System;
using System.Collections.Generic;
using System.Drawing;
using System.Drawing.Printing;
using System.IO;
using System.Runtime.InteropServices;
using System.Text;

public sealed class WeikongLabelItem
{
    public string Sn { get; set; }
    public string BindCode { get; set; }
    public string QrBase64 { get; set; }
}

public static class WeikongLabelPrinter
{
    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    private sealed class DOC_INFO_1
    {
        [MarshalAs(UnmanagedType.LPWStr)] public string pDocName;
        [MarshalAs(UnmanagedType.LPWStr)] public string pOutputFile;
        [MarshalAs(UnmanagedType.LPWStr)] public string pDataType;
    }

    [DllImport("winspool.drv", SetLastError = true, CharSet = CharSet.Unicode)]
    private static extern bool OpenPrinter(string printerName, out IntPtr printer, IntPtr defaults);
    [DllImport("winspool.drv", SetLastError = true)]
    private static extern bool ClosePrinter(IntPtr printer);
    [DllImport("winspool.drv", SetLastError = true, CharSet = CharSet.Unicode)]
    private static extern int StartDocPrinter(IntPtr printer, int level, [In] DOC_INFO_1 docInfo);
    [DllImport("winspool.drv", SetLastError = true)]
    private static extern bool EndDocPrinter(IntPtr printer);
    [DllImport("winspool.drv", SetLastError = true)]
    private static extern bool StartPagePrinter(IntPtr printer);
    [DllImport("winspool.drv", SetLastError = true)]
    private static extern bool EndPagePrinter(IntPtr printer);
    [DllImport("winspool.drv", SetLastError = true)]
    private static extern bool WritePrinter(IntPtr printer, byte[] bytes, int count, out int written);

    private static int MmToHundredthsInch(double mm)
    {
        return (int)Math.Round(mm / 25.4 * 100.0);
    }

    public static string[] GetPrinters()
    {
        var values = new List<string>();
        foreach (string printer in PrinterSettings.InstalledPrinters) values.Add(printer);
        return values.ToArray();
    }

    public static void Print(string printerName, double widthMm, double heightMm, int copies, WeikongLabelItem[] items)
    {
        if (items == null || items.Length == 0) throw new ArgumentException("No labels supplied.");
        if (copies < 1 || copies > 20) throw new ArgumentOutOfRangeException("copies");

        if (printerName.IndexOf("GZPL", StringComparison.OrdinalIgnoreCase) >= 0)
        {
            PrintGzpl(printerName, widthMm, heightMm, copies, items);
            return;
        }

        var queue = new List<WeikongLabelItem>();
        foreach (var item in items)
            for (var copy = 0; copy < copies; copy++) queue.Add(item);

        var index = 0;
        using (var document = new PrintDocument())
        {
            document.DocumentName = "Weikong gateway labels";
            document.PrintController = new StandardPrintController();
            document.PrinterSettings.PrinterName = printerName;
            if (!document.PrinterSettings.IsValid) throw new InvalidOperationException("Printer is unavailable: " + printerName);

            document.DefaultPageSettings.Margins = new Margins(0, 0, 0, 0);
            document.DefaultPageSettings.PaperSize = new PaperSize(
                "Weikong " + widthMm + "x" + heightMm + "mm",
                MmToHundredthsInch(widthMm),
                MmToHundredthsInch(heightMm));
            document.DefaultPageSettings.Landscape = false;

            document.PrintPage += (sender, args) =>
            {
                var item = queue[index++];
                args.Graphics.PageUnit = GraphicsUnit.Millimeter;
                args.Graphics.Clear(Color.White);
                args.Graphics.TextRenderingHint = System.Drawing.Text.TextRenderingHint.ClearTypeGridFit;

                var padding = 2f;
                var qrSize = Math.Max(22f, (float)heightMm - padding * 2);
                var qrBytes = Convert.FromBase64String(item.QrBase64);
                using (var stream = new MemoryStream(qrBytes))
                using (var image = Image.FromStream(stream))
                {
                    args.Graphics.InterpolationMode = System.Drawing.Drawing2D.InterpolationMode.NearestNeighbor;
                    args.Graphics.DrawImage(image, padding, padding, qrSize, qrSize);
                }

                var left = padding + qrSize + 1.8f;
                var available = Math.Max(8f, (float)widthMm - left - padding);
                using (var brandFont = new Font("Microsoft YaHei", heightMm >= 40 ? 10f : 8f, FontStyle.Bold, GraphicsUnit.Point))
                using (var smallFont = new Font("Microsoft YaHei", heightMm >= 40 ? 7f : 6f, FontStyle.Regular, GraphicsUnit.Point))
                using (var snFont = new Font("Consolas", heightMm >= 40 ? 8f : 6.5f, FontStyle.Bold, GraphicsUnit.Point))
                using (var codeFont = new Font("Consolas", heightMm >= 40 ? 15f : 11f, FontStyle.Bold, GraphicsUnit.Point))
                using (var dark = new SolidBrush(Color.FromArgb(17, 24, 39)))
                using (var muted = new SolidBrush(Color.FromArgb(100, 116, 139)))
                {
                    args.Graphics.DrawString("微控网关", brandFont, dark, new RectangleF(left, 2.5f, available, 6f));
                    args.Graphics.DrawString("扫码绑定设备", smallFont, muted, new RectangleF(left, 8.5f, available, 5f));
                    args.Graphics.DrawString(item.Sn ?? "", snFont, dark, new RectangleF(left, 14f, available, heightMm >= 40 ? 8f : 6f));
                    args.Graphics.DrawString("绑定码", smallFont, muted, new RectangleF(left, heightMm >= 40 ? 23f : 20f, available, 5f));
                    args.Graphics.DrawString(item.BindCode ?? "", codeFont, dark, new RectangleF(left, heightMm >= 40 ? 28f : 23f, available, 9f));
                }
                args.HasMorePages = index < queue.Count;
            };
            document.Print();
        }
    }

    private static string ZplSafe(string value)
    {
        return (value ?? string.Empty).Replace("^", string.Empty).Replace("~", string.Empty);
    }

    private static void PrintGzpl(string printerName, double widthMm, double heightMm, int copies, WeikongLabelItem[] items)
    {
        const double dotsPerMm = 8.0; // G500: 203 dpi
        int width = Math.Max(240, (int)Math.Round(widthMm * dotsPerMm));
        int height = Math.Max(160, (int)Math.Round(heightMm * dotsPerMm));
        // BQN size is based on QR modules, not the requested drawing box.
        // Keep the magnification conservative so the generated QR never
        // extends into the text column on small labels.
        int qrMagnification = heightMm <= 30 ? 4 : (heightMm <= 40 ? 5 : 6);
        int estimatedQrDots = qrMagnification * 49;
        int qrY = Math.Max(8, (height - estimatedQrDots) / 2);
        // Reserve a stable 58% QR column. The QR version grows with payload
        // length, so estimating it with a fixed module count can overlap text.
        int textX = Math.Min(width - 150, (int)Math.Round(width * 0.58));
        int textWidth = Math.Max(130, width - textX - 12);
        // Scale and vertically centre the information block for every label
        // height. The old fixed Y coordinates used only the upper half of a
        // 70x50 label and made the physical result look top-heavy.
        double scale = Math.Max(0.68, Math.Min(1.0, height / 400.0));
        int blockHeight = (int)Math.Round(276 * scale);
        int textTop = Math.Max(8, (height - blockHeight) / 2);
        int brandHeight = (int)Math.Round(32 * scale);
        int brandWidth = (int)Math.Round(27 * scale);
        int smallHeight = (int)Math.Round(19 * scale);
        int smallWidth = (int)Math.Round(16 * scale);
        int snHeight = (int)Math.Round(20 * scale);
        int snWidth = (int)Math.Round(14 * scale);
        int codeHeight = (int)Math.Round(38 * scale);
        int codeWidth = (int)Math.Round(32 * scale);
        var output = new StringBuilder();
        foreach (var item in items)
        {
            string sn = ZplSafe(item.Sn);
            string bindCode = ZplSafe(item.BindCode);
            string payload = ZplSafe("{\"type\":\"weikong-gateway-bind\",\"version\":1,\"sn\":\"" + sn + "\",\"bindCode\":\"" + bindCode + "\"}");
            output.Append("^XA");
            output.Append("^PW").Append(width).Append("^LL").Append(height).Append("^LH0,0^CI27");
            output.Append("^FO12,").Append(qrY).Append("^BQN,2,").Append(qrMagnification).Append("^FDLA,").Append(payload).Append("^FS");
            output.Append("^FO").Append(textX).Append(",").Append(textTop).Append("^A0N,").Append(brandHeight).Append(",").Append(brandWidth).Append("^FB").Append(textWidth).Append(",1,0,L,0^FDWEIKONG^FS");
            output.Append("^FO").Append(textX).Append(",").Append(textTop + (int)Math.Round(45 * scale)).Append("^A0N,").Append(smallHeight).Append(",").Append(smallWidth).Append("^FDSCAN TO BIND^FS");
            output.Append("^FO").Append(textX).Append(",").Append(textTop + (int)Math.Round(100 * scale)).Append("^A0N,").Append(smallHeight).Append(",").Append(smallWidth).Append("^FDSN^FS");
            output.Append("^FO").Append(textX).Append(",").Append(textTop + (int)Math.Round(128 * scale)).Append("^A0N,").Append(snHeight).Append(",").Append(snWidth).Append("^FB").Append(textWidth).Append(",2,2,L,0^FD").Append(sn).Append("^FS");
            output.Append("^FO").Append(textX).Append(",").Append(textTop + (int)Math.Round(205 * scale)).Append("^A0N,").Append(smallHeight).Append(",").Append(smallWidth).Append("^FDBIND CODE^FS");
            output.Append("^FO").Append(textX).Append(",").Append(textTop + (int)Math.Round(236 * scale)).Append("^A0N,").Append(codeHeight).Append(",").Append(codeWidth).Append("^FB").Append(textWidth).Append(",1,0,L,0^FD").Append(bindCode).Append("^FS");
            output.Append("^PQ").Append(copies).Append(",0,1,N^XZ");
        }
        SendRaw(printerName, Encoding.ASCII.GetBytes(output.ToString()));
    }

    private static void SendRaw(string printerName, byte[] bytes)
    {
        IntPtr printer;
        if (!OpenPrinter(printerName, out printer, IntPtr.Zero))
            throw new InvalidOperationException("Cannot open printer: " + printerName + " (" + Marshal.GetLastWin32Error() + ")");
        try
        {
            var info = new DOC_INFO_1 { pDocName = "Weikong gateway GZPL labels", pDataType = "RAW", pOutputFile = null };
            if (StartDocPrinter(printer, 1, info) == 0)
                throw new InvalidOperationException("Cannot start RAW print job (" + Marshal.GetLastWin32Error() + ")");
            try
            {
                if (!StartPagePrinter(printer)) throw new InvalidOperationException("Cannot start print page (" + Marshal.GetLastWin32Error() + ")");
                try
                {
                    int written;
                    if (!WritePrinter(printer, bytes, bytes.Length, out written) || written != bytes.Length)
                        throw new InvalidOperationException("RAW print write failed (" + Marshal.GetLastWin32Error() + ")");
                }
                finally { EndPagePrinter(printer); }
            }
            finally { EndDocPrinter(printer); }
        }
        finally { ClosePrinter(printer); }
    }
}
