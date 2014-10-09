function clean_input(i) {
    return i.replace(/^\s+|\s+$/g, '');
}

function clean_up(str){
    return clean_input(str).toUpperCase();
}

function dockerfile_log(level, item, errors)
{
	var logUrl = '/tutorial/api/dockerfile_event/';
	$.ajax({
			url: logUrl,
			type: "POST",
			cache:false,
			data: {
				'errors': errors,
				'level': level,
				'item': item,
			},
		}).done( function() { } );
}

function validate_email(email)
{ 
	var re = /^(([^<>()[\]\\.,;:\s@\"]+(\.[^<>()[\]\\.,;:\s@\"]+)*)|(\".+\"))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\])|(([a-zA-Z\-0-9]+\.)+[a-zA-Z]{2,}))$/;
	return re.test(email);
} 

$(document).ready(function() {

    /* prepare to send the csrf-token on each ajax-request */
    var csrftoken = $.cookie('csrftoken');
    $.ajaxSetup({
        headers: { 'X-CSRFToken': csrftoken }
    });

    $("#send_email").click( function()
    {
        $('#email_invalid').hide();
        $('#email_already_registered').hide();
        $('#email_registered').hide();

        email = $('#email').val();
        if (!validate_email(email))
        {
            $('#email_invalid').show();
            return (false);
        }

        var emailUrl = '/tutorial/api/subscribe/';

        $.ajax({
                url: emailUrl,
                type: "POST",
                cache:false,
                data: {
                    'email': email,
                    'from_level': $(this).data('level')
                },
            }).done( function(data ) {
                    if (data == 1) // already registered
                    {
                        $('#email_already_registered').show();
                    }
                    else if (data == 0) // registered ok
                    {
                        $('#email_registered').show();
                    }

                } );
        return (true);
    });
})
