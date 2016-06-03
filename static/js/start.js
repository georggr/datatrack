$(document).ready(function() {
	
	var ifUploadedGoogle = false;
	var ifUploadedFacebook = false;
	
	$("#start").show();

	$('#fileToUpload').change(function() {
		$('#toProcessing').prop('disabled', !($('#fileToUpload').val()));
	});

	$.get("/v1/disclosure/toOrganization/Google/count", function(disclosure) {  //check if file has been uploaded
		if(disclosure != 0)
			ifUploadedGoogle  = true;
	});
	$.get("/v1/disclosure/toOrganization/Facebook/count", function(disclosure) {  //check if file has been uploaded
		if(disclosure != 0)
			ifUploadedFacebook  = true;
	});
	
	$("#back").click(function() {
		
		$("#import").toggleClass("hide showme");
		$("#uploadpart").toggleClass("hide showme");
		$("#processing-error").toggleClass("hide showme");

	});

	
	function showToImport(ifUploaded) {
		$("#import").toggleClass("hide showme");
		if(!ifUploaded)
		{
			$("#uploadpart").toggleClass("hide showme");

		}
		else 
		{
			$("#textcomplete").toggleClass("hide showme");
			$("#nextstep").click(function() {
				$("#uploadpart").toggleClass("hide showme");
				$("#import").toggleClass("hide showme");
				$("#done").toggleClass("hide showme");
			});
		}
	}

	$("#to-import-google").click(function() {
		$("#start").hide();
		$("#importform").attr("action", "google");
		$(".facebook-explanation").addClass("hide showme");
		$(".google-explanation").removeClass("hide showme");
		$(".provider-name").text("Google");
		$(".provider-data-name").text("Google Takeout");
		showToImport(ifUploadedGoogle);
	});
	$("#to-import-facebook").click(function() {
		$("#start").hide();
		$("#importform").attr("action", "facebook");
		$(".facebook-explanation").removeClass("hide showme");
		$(".google-explanation").addClass("hide showme");
		$(".provider-name").text("Facebook");
		$(".provider-data-name").text("Facebook data");
		showToImport(ifUploadedFacebook);
	});

	$("#importform").submit(function(event) {
		event.stopPropagation();
		event.preventDefault();
		$("#uploadpart").toggleClass("hide showme");
		$("#import").toggleClass("hide showme");
		$("#processing").toggleClass("hide showme");

		var formData = new FormData($(this)[0]);

		$.ajax({
			url: "/v1/" + $("#importform").attr("action"),
			type: "POST",
			data: formData,
			success: function() {
				$("#processing").toggleClass("hide showme");
				$("#done").toggleClass("hide showme");
			},
			error: function() { 
				//alert("error"); 
				$("#processing").toggleClass("hide showme");

				$("#processing-error").toggleClass("hide showme");				
			},
			contentType: false,
			cache: false,
			processData: false
		});
	});
});

function importComplete() {
	alert("done");
}

function importError() {
	alert("ohnoe");
}
