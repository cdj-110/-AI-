let idTmr; //保存定时器

let getExplorer = () => {
	let explorer = window.navigator.userAgent;
	//ie
	if (explorer.indexOf("MSIE") >= 0) {
		return 'ie';
	}
	//firefox

	else if (explorer.indexOf("Firefox") >= 0) {
		return 'Firefox';
	}
	//Chrome
	else if (explorer.indexOf("Chrome") >= 0) {
		return 'Chrome';
	}
	//Opera
	else if (explorer.indexOf("Opera") >= 0) {
		return 'Opera';
	}
	//Safari
	else if (explorer.indexOf("Safari") >= 0) {
		return 'Safari';
	}
}
// 判断浏览器是否为IE
let exportToExcel = (data, name) => {

	// 判断是否为IE
	if (getExplorer() == 'ie') {
		tableToIE(data, name)
	} else {
		tableToNotIE(data, name)
	}
}

let Cleanup = () => {
	window.clearInterval(idTmr);
}

// ie浏览器下执行
let tableToIE = (data, name) => {
	let curTbl = data;
	let oXL = new ActiveXObject("Excel.Application");

	//创建AX对象excel
	let oWB = oXL.Workbooks.Add();
	//获取workbook对象
	let xlsheet = oWB.Worksheets(1);
	//激活当前sheet
	let sel = document.body.createTextRange();
	sel.moveToElementText(curTbl);
	//把表格中的内容移到TextRange中
	sel.select;
	//全选TextRange中内容
	sel.execCommand("Copy");
	//复制TextRange中内容
	xlsheet.Paste();
	//粘贴到活动的EXCEL中

	oXL.Visible = true;
	//设置excel可见属性

	try {
		let fname = oXL.Application.GetSaveAsFilename("Excel.xls", "Excel Spreadsheets (*.xls), *.xls");
	} catch (e) {
		print("Nested catch caught " + e);
	} finally {
		oWB.SaveAs(fname);

		oWB.Close(savechanges = false);
		//xls.visible = false;
		oXL.Quit();
		oXL = null;
		// 结束excel进程，退出完成
		window.setInterval("Cleanup();", 1);
		idTmr = window.setInterval("Cleanup();", 1);
	}
}

// 非ie浏览器下执行
let tableToNotIE = (function () {
	// 编码要用utf-8不然默认gbk会出现中文乱码
	let uri = 'data:application/vnd.ms-excel;base64,',
		template = '<html xmlns:o="urn:schemas-microsoft-com:office:office" xmlns:x="urn:schemas-microsoft-com:office:excel" xmlns="http://www.w3.org/TR/REC-html40"><head><meta charset="UTF-8"><!--[if gte mso 9]><xml><x:ExcelWorkbook><x:ExcelWorksheets><x:ExcelWorksheet><x:Name>{worksheet}</x:Name><x:WorksheetOptions><x:DisplayGridlines/></x:WorksheetOptions></x:ExcelWorksheet></x:ExcelWorksheets></x:ExcelWorkbook></xml><![endif]--></head><body><table>{table}</table></body></html>';

		let base64 = function (s) {
		return window.btoa(unescape(encodeURIComponent(s)));
	}

	let format = (s, c) => {
		return s.replace(/{(\w+)}/g,
			(m, p) => {
				return c[p];
			})
	}

	return (table, name) => {
		let ctx = {
			worksheet: name,
			table
		}

		let url = uri + base64(format(template, ctx));

		if (navigator.userAgent.indexOf("Firefox") > -1){
			window.location.href = url
		} else {
			let aLink = document.createElement('a');
			aLink.href = url;
			aLink.download = name || '';
			let event;
			if (window.MouseEvent) {
				event = new MouseEvent('click');
			} else {
				event = document.createEvent('MouseEvents');
				event.initMouseEvent('click', true, false, window, 0, 0, 0, 0, 0, false, false, false, false, 0, null);
			}
			aLink.dispatchEvent(event);
		}
	}
})()

let resolveOptions = (options) => {
	if (options.length === 1) {
		return options[0]
	}

	return {
		column: options[0] || [],
		data: options[1] || [],
		excelName: options[2] || '',
		captionName: options[3],
	}
}

/**
 * 导出函数
 * @inner(options) 接收表头、表格数据、文件名称三个参数
 * @使用示例
 * exportExeclFunctionsEncapsulatedByDaiCiMeiGui([
        { type: "text", key: "a1", title: "位置信息"},
        { type: "text", key: "a2", title: "设备名称"},
    ], tableData.value, "设备管理 - 设备档案");
 */
let exportExeclFunctionsEncapsulatedByDaiCiMeiGui = (...options) => {
	function getTextHtml(val) {
		return `<td style="text-align: center">${val}</td>`
	}

	function getImageHtml(val, options) {
		options = Object.assign({width: 40, height: 60}, options)
		return `<td style="width: ${options.width}px; height: ${options.height}px; text-align: center; vertical-align: middle"><img src="${val}" width=${ options.width * .99 } height=${ options.height * .99 } /></td>`
	}

	let typeMap = {
		image: getImageHtml,
		text: getTextHtml
	}

	let {
		column,
		data,
		excelName,
		captionName,
	} = resolveOptions(options)

	let caption = captionName ? `<caption style="font-weight:bold">${captionName}</caption>` : '';

	let thead = column.reduce((result, item) => {
		result += `<th>${item.title}</th>`
		return result
	}, '')

	thead = `<thead><tr>${thead}</tr></thead>`

	let tbody = data.reduce((result, row) => {
		let temp = column.reduce((tds, col) => {
			let options = {}

			if (col.type === 'image') {
				if (row.size) {
					options.width = row.size[0]
					options.height = row.size[1]
				} else {
					col.width && (options.width = col.width)
					col.height && (options.height = col.height)
				}
			}

			tds += typeMap[col.type || 'text'](row[col.key], options)

			return tds
		}, '')

		result += `<tr>${temp}</tr>`

		return result
	}, '')

	tbody = `<tbody>${tbody}</tbody>`

	let table = caption + thead + tbody

	// 导出表格
	exportToExcel(table, excelName)
}

export default exportExeclFunctionsEncapsulatedByDaiCiMeiGui;