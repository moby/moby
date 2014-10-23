function check_form1 ()
{
	$('#level1_error0').hide();
	$('#level1_error1').hide();
	$('#level1_error2').hide();
	$('#level1_error3').hide();
	
	$('#no_good').hide();	
	$('#some_good').hide();		
	$('#all_good').hide();

	var a = clean_input($('#level1_q0').val()).toUpperCase();
	var b = clean_input($('#level1_q1').val()).toUpperCase();
	var c = clean_input($('#level1_q2').val()).toUpperCase();
	var d = clean_input($('#level1_q3').val());
	var points = 0;
	
	if (a == 'FROM'){
		points = points + 1;
	} else {
		$('#level1_error0').show();
	}
	if (b == 'RUN') {
		points = points + 1;
	} else {
		$('#level1_error1').show();
	}
	if (c == 'MAINTAINER') {
		points = points + 1;
	} else {
		$('#level1_error2').show();
	}
	if (d == '#') {
		points = points + 1;
	} else {
		$('#level1_error3').show();
	}
	if (points == 4) {// all good
		$('#all_good').show();
	} else if (points == 0) { // nothing good
		$('#no_good').show();	
	} else {// some good some bad
		$('#some_good').show();
	}
	return (4 - points);
}

function check_form2 ()
{
    $('.level_questions .alert').hide();

    var answers = {};
    answers[0] = ['FROM'];
    answers[1] = ['ENTRYPOINT', 'CMD'];
    answers[2] = ['#'];
    answers[3] = ['USER'];
    answers[4] = ['RUN'];
    answers[5] = ['EXPOSE'];
    answers[6] = ['MAINTAINER'];
    answers[7] = ['ENTRYPOINT', 'CMD'];

	var points = 0;

    $.each($(".level"), function(num, input){
        var cleaned = clean_up(input.value);
        if ($.inArray(cleaned, answers[num]) == -1) {
            $( $(".level_error")[num]).show()
            $(input).addClass("error_input");
        } else {
            $( $(".level_error")[num]).hide()
            $(input).removeClass("error_input");
            points += 1;
        }
    })
	if (points == 8) // all good
	{
		$('#all_good').show();
	}
	else if (points == 0) // nothing good
	{
		$('#no_good').show();
	}
	else // some good some bad
	{
		$('#some_good').show();
	}
	return (8- points);
}

function check_fill(answers)
{
	$('#dockerfile_ok').hide();
	$('#dockerfile_ko').hide();

	var errors = 0;

    $.each($(".l_fill"), function(num, input){
        var cleaned = clean_up(input.value);
        var id = input.id;
        if (answers[id] != cleaned) {
            $(input).addClass("error_input");
            errors += 1;
        } else {
            $(input).removeClass("error_input");
        }
    });

	if (errors != 0)
	{
		$('#dockerfile_ko').show();
	}
	else
	{
		$('#dockerfile_ok').show();
	}
	return (errors);
}

$(document).ready(function() {

    $("#check_level1_questions").click( function(){
        errors = check_form1();
        dockerfile_log(1, '1_questions', errors);
       }
    );

    $("#check_level1_fill").click( function(){
        var answers = {};
        answers['from'] = 'FROM';
        answers['ubuntu'] = 'UNTU';
        answers['maintainer'] = 'MAINTAINER';
        answers['eric'] = 'RIC';
        answers['bardin'] = 'ARDIN';
        answers['run0'] = 'RUN';
        answers['run1'] = 'RUN';
        answers['run2'] = 'RUN';
        answers['memcached'] = 'MEMCACHED';

        var errors = check_fill(answers);
        dockerfile_log(1, '2_fill', errors);
    });

    $("#check_level2_questions").click( function(){
        errors = check_form2();
        dockerfile_log(2, '1_questions', errors);
       }
    );

    $("#check_level2_fill").click( function(){
        var answers = {};
        answers['from'] = "FROM";
        answers['ubuntu'] = "UNTU";
        answers['maintainer'] = "AINER";
        answers['roberto'] = "BERTO";
        answers['hashioka'] = "SHIOKA";
        answers['run0'] = "RUN";
        answers['run1'] = "RUN";
        answers['run2'] = "RUN";
        answers['run3'] = "RUN";
        answers['run4'] = "RUN";
        answers['run5'] = "RUN";
        answers['run6'] = "RUN";
        answers['entrypoint'] = "ENTRYPOINT";
        answers['user'] = "USER";
        answers['expose'] = "EXPOSE";
        answers['gcc'] = "GCC";

        var errors = check_fill(answers);
        dockerfile_log(2, '2_fill', errors);
    });

    $(".btn.btn-primary.back").click( function(event){
        event.preventDefault();
        window.history.back();
    })
});
